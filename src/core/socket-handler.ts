import makeWASocket, {
  DisconnectReason,
  WASocket,
  ConnectionState,
  fetchLatestBaileysVersion
} from '@whiskeysockets/baileys';
import { Boom } from '@hapi/boom';
import { logger } from '../utils/logger';
import { usePostgresAuthState } from './auth-state';
import { sessionRepository } from '../repositories/session.repository';
import { dispatchWebhook } from './event-dispatcher';
import { redis } from '../config/redis';

export const startSocket = async (
  sessionId: string,
  onQR?: (qr: string) => void,
  onConnected?: () => void,
  onDisconnected?: (reason: number) => void
): Promise<WASocket> => {
  const { state, saveCreds, clearState } = await usePostgresAuthState(sessionId);
  const { version } = await fetchLatestBaileysVersion();

  const sock = makeWASocket({
    version,
    auth: state,
    printQRInTerminal: false,
    logger: logger.child({ module: 'baileys', sessionId }) as any,
    browser: ['WSBot', 'Chrome', '1.0.0'],
    syncFullHistory: false,
    markOnlineOnConnect: true,
  });

  sock.ev.on('creds.update', saveCreds);

  sock.ev.on('connection.update', async (update: Partial<ConnectionState>) => {
    const { connection, lastDisconnect, qr } = update;

    if (qr && onQR) {
      onQR(qr);
    }

    if (connection === 'close') {
      const statusCode = (lastDisconnect?.error as Boom)?.output?.statusCode;
      const shouldReconnect = statusCode !== DisconnectReason.loggedOut;
      
      logger.info({ sessionId, statusCode, shouldReconnect }, 'Connection closed');

      if (statusCode === DisconnectReason.loggedOut) {
        await clearState();
        await sessionRepository.updateStatus(sessionId, 'LOGGED_OUT');
        if (onDisconnected) onDisconnected(statusCode);
      } else {
        await sessionRepository.updateStatus(sessionId, 'DISCONNECTED');
        if (onDisconnected) onDisconnected(statusCode || 500);
      }
    } else if (connection === 'open') {
      logger.info({ sessionId }, 'Connection opened');
      await sessionRepository.updateStatus(sessionId, 'CONNECTED', sock.user?.id.split(':')[0]);
      if (onConnected) onConnected();
    }
  });

  sock.ev.on('messages.upsert', async (m) => {
    dispatchWebhook(sessionId, 'messages.upsert', m);

    // Cache incoming media messages for 24 hours so they can be downloaded via REST endpoint
    for (const msg of m.messages) {
      if (!msg.message) continue;
      
      const msgContent = msg.message.imageMessage || msg.message.videoMessage || msg.message.audioMessage || msg.message.documentMessage || msg.message.stickerMessage;
      
      if (msgContent && msg.key.id) {
        // Cache full WAMessage object
        await redis.set(`message:${sessionId}:${msg.key.id}`, JSON.stringify(msg), 'EX', 86400);
      }
    }
  });

  sock.ev.on('messages.update', (m) => {
    dispatchWebhook(sessionId, 'messages.update', m);
  });

  sock.ev.on('message-receipt.update', (m) => {
    dispatchWebhook(sessionId, 'message-receipt.update', m);
  });

  sock.ev.on('presence.update', (m) => {
    dispatchWebhook(sessionId, 'presence.update', m);
  });

  sock.ev.on('contacts.update', (m) => {
    dispatchWebhook(sessionId, 'contacts.update', m);
  });

  return sock;
};
