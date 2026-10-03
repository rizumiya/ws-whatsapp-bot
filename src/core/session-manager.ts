import { WASocket } from '@whiskeysockets/baileys';
import { startSocket } from './socket-handler';
import { logger } from '../utils/logger';
import { sessionRepository } from '../repositories/session.repository';
import { redis } from '../config/redis';

const MAX_RECONNECT = 5;

/** 'socket' = socket hidup, 'pending' = sedang dibuka / menunggu sambung ulang, 'stopped' = tidak ada. */
export type RuntimeState = 'socket' | 'pending' | 'stopped';

class SessionManager {
  private sessions: Map<string, WASocket> = new Map();
  // Sesi yang socket-nya sedang dibuka (belum masuk `sessions`).
  private starting: Set<string> = new Set();
  // Sesi yang menunggu jadwal sambung ulang.
  private reconnectTimers: Map<string, NodeJS.Timeout> = new Map();

  public async createSession(sessionId: string, apiKey: string): Promise<WASocket> {
    if (this.sessions.has(sessionId) || this.starting.has(sessionId)) {
      throw new Error(`Session ${sessionId} is already active.`);
    }
    // Ditandai sebelum await pertama, supaya status runtime langsung 'pending' begitu API menjawab.
    this.starting.add(sessionId);
    this.cancelReconnect(sessionId);

    try {
      const existing = await sessionRepository.get(sessionId);
      if (!existing) {
        await sessionRepository.create(sessionId, apiKey);
      } else {
        await sessionRepository.updateStatus(sessionId, 'STARTING');
      }
      // QR dari socket sebelumnya sudah tidak berlaku.
      await redis.del(`session:${sessionId}:qr`);

      return await this.initializeSocket(sessionId);
    } catch (error) {
      this.starting.delete(sessionId);
      throw error;
    }
  }

  private async initializeSocket(sessionId: string, retryCount = 0): Promise<WASocket> {
    // Socket yang pernah tersambung lalu putus mulai menghitung percobaan dari nol lagi;
    // tanpa ini sesi yang hidup berminggu-minggu menyerah setelah beberapa kali putus.
    let wasConnected = false;
    let sock: WASocket;

    this.starting.add(sessionId);
    try {
      sock = await startSocket(
        sessionId,
        async (qr) => {
          // Cache QR in Redis with 60s TTL
          await redis.set(`session:${sessionId}:qr`, qr, 'EX', 60);
          await sessionRepository.updateStatus(sessionId, 'QR_PENDING');
        },
        () => {
          wasConnected = true;
          this.sessions.set(sessionId, sock);
          redis.del(`session:${sessionId}:qr`).catch(() => undefined);
          logger.info({ sessionId }, 'Session fully registered in manager');
        },
        (reason) => {
          // Socket ini sudah diganti atau sengaja dihentikan (restart / logout / hapus):
          // jangan disambungkan ulang, nanti satu nomor punya dua socket yang saling menendang.
          if (this.sessions.get(sessionId) !== sock) return;
          this.sessions.delete(sessionId);

          // Handle reconnect if not logged out (401)
          if (reason === 401) return;

          const attempt = wasConnected ? 0 : retryCount;
          if (attempt >= MAX_RECONNECT) {
            logger.error({ sessionId }, 'Max reconnect retries reached');
            return;
          }

          const backoff = Math.min(1000 * Math.pow(2, attempt), 30000); // Max 30s
          logger.info({ sessionId, backoff, retryCount: attempt }, 'Scheduling reconnect');
          const timer = setTimeout(() => {
            this.reconnectTimers.delete(sessionId);
            this.reconnect(sessionId, attempt + 1).catch(err => {
              logger.error(err, `Reconnect failed for ${sessionId}`);
            });
          }, backoff);
          this.reconnectTimers.set(sessionId, timer);
        }
      );
    } finally {
      this.starting.delete(sessionId);
    }

    this.sessions.set(sessionId, sock);
    return sock;
  }

  private async reconnect(sessionId: string, retryCount: number): Promise<void> {
    // Selama menunggu, sesi bisa sudah dibuka lagi lewat API atau dihapus.
    if (this.sessions.has(sessionId) || this.starting.has(sessionId)) return;
    if (!(await sessionRepository.get(sessionId))) return;
    await this.initializeSocket(sessionId, retryCount);
  }

  private cancelReconnect(sessionId: string): void {
    const timer = this.reconnectTimers.get(sessionId);
    if (timer) {
      clearTimeout(timer);
      this.reconnectTimers.delete(sessionId);
    }
  }

  public getSession(sessionId: string): WASocket | undefined {
    return this.sessions.get(sessionId);
  }

  public runtimeState(sessionId: string): RuntimeState {
    if (this.sessions.has(sessionId)) return 'socket';
    if (this.starting.has(sessionId) || this.reconnectTimers.has(sessionId)) return 'pending';
    return 'stopped';
  }

  public async deleteSession(sessionId: string, logout = true): Promise<void> {
    this.cancelReconnect(sessionId);
    const sock = this.sessions.get(sessionId);
    // Dikeluarkan dari daftar DULU, supaya penutupan socket di bawah tidak memicu sambung ulang.
    this.sessions.delete(sessionId);

    if (sock) {
      if (logout) {
        try {
          await sock.logout();
        } catch (error: any) {
          logger.warn({ sessionId, error: error?.message }, 'Logout failed, closing socket instead');
          sock.end(undefined);
        }
      } else {
        sock.end(undefined);
      }
    }

    if (!logout) {
      await sessionRepository.updateStatus(sessionId, 'DISCONNECTED');
    }
  }

  /** Logout dari WhatsApp (bila tersambung), lalu hapus sesi beserta kredensial, log, dan cache-nya. */
  public async removeSession(sessionId: string): Promise<void> {
    await this.deleteSession(sessionId, true);
    // ON DELETE CASCADE ikut menghapus session_auth_keys, webhook_logs, dan message_logs.
    await sessionRepository.delete(sessionId);

    // sessionId hanya [a-zA-Z0-9_-], jadi aman dipakai di pola KEYS.
    const keys = [
      ...(await redis.keys(`auth:${sessionId}:*`)),
      ...(await redis.keys(`message:${sessionId}:*`)),
      `session:${sessionId}:qr`,
    ];
    await redis.del(...keys);
  }

  public async restoreSessions(): Promise<void> {
    try {
      const sessions = await sessionRepository.getAll();
      // Restore sessions that were previously connected
      const activeSessions = sessions.filter(s => s.status === 'CONNECTED');

      logger.info(`Found ${activeSessions.length} active sessions to restore...`);

      for (let i = 0; i < activeSessions.length; i++) {
        const s = activeSessions[i];
        try {
          await this.initializeSocket(s.id);
          // Batched start: delay 500ms between initializations to prevent CPU spikes
          await new Promise(resolve => setTimeout(resolve, 500));
        } catch (error) {
          logger.error(error, `Failed to restore session ${s.id}`);
        }
      }
    } catch (error) {
      logger.error(error, 'Failed to fetch sessions from database for restoration');
    }
  }
}

export const sessionManager = new SessionManager();
