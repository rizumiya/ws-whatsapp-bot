import { AnyMessageContent, delay } from '@whiskeysockets/baileys';
import dns from 'dns/promises';
import net from 'net';
import { dbQuery } from '../config/database';
import { sessionManager } from '../core/session-manager';
import { logger } from '../utils/logger';

const MAX_REMOTE_MEDIA_BYTES = 20 * 1024 * 1024; // 20 MB
const FETCH_TIMEOUT_MS = 15000; // 15 seconds

const isPrivateOrReservedIP = (ip: string): boolean => {
  if (net.isIPv4(ip)) {
    const parts = ip.split('.').map(Number);
    const [a, b] = parts;
    if (
      a === 0 ||
      a === 10 ||
      a === 127 ||
      (a === 100 && b >= 64 && b <= 127) ||
      (a === 169 && b === 254) ||
      (a === 172 && b >= 16 && b <= 31) ||
      (a === 192 && b === 168) ||
      a >= 224
    ) {
      return true;
    }
    return false;
  }
  if (net.isIPv6(ip)) {
    const norm = ip.toLowerCase();
    if (
      norm === '::1' ||
      norm === '::' ||
      norm.startsWith('fc') ||
      norm.startsWith('fd') ||
      norm.startsWith('fe8') ||
      norm.startsWith('fe9') ||
      norm.startsWith('fea') ||
      norm.startsWith('feb')
    ) {
      return true;
    }
    if (norm.startsWith('::ffff:')) {
      return isPrivateOrReservedIP(norm.slice(7));
    }
  }
  return false;
};

const assertSafeExternalUrl = async (rawUrl: string): Promise<URL> => {
  let parsed: URL;
  try {
    parsed = new URL(rawUrl);
  } catch {
    throw new Error('Invalid media URL');
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    throw new Error('Only http and https media URLs are allowed');
  }
  const hostname = parsed.hostname.replace(/^\[|\]$/g, '');
  if (hostname === 'localhost' || hostname.endsWith('.local') || hostname.endsWith('.internal')) {
    throw new Error('Fetching media from local/internal hostnames is not allowed');
  }
  if (net.isIP(hostname)) {
    if (isPrivateOrReservedIP(hostname)) {
      throw new Error('Fetching media from private or loopback IP addresses is not allowed');
    }
  } else {
    const records = await dns.lookup(hostname, { all: true });
    if (!records.length || records.some((r) => isPrivateOrReservedIP(r.address))) {
      throw new Error('Media URL resolves to a private or disallowed IP address');
    }
  }
  return parsed;
};

export const messagingService = {
  async sendMessage(
    sessionId: string,
    to: string,
    content: AnyMessageContent,
    options?: any
  ) {
    const sock = sessionManager.getSession(sessionId);
    if (!sock || !sock.user) {
      throw new Error(`Session ${sessionId} is not connected`);
    }

    const isActionOnly = 'react' in content || 'delete' in content;
    if (!isActionOnly) {
      // Optional: Add small delay for human-like presence on content messages
      try {
        await sock.presenceSubscribe(to);
        await delay(300);
        await sock.sendPresenceUpdate('composing', to);
        await delay(600);
        await sock.sendPresenceUpdate('paused', to);
      } catch {
        // Presence update can fail on newsletters or certain JIDs; do not block message sending
      }
    }

    const sentMsg = await sock.sendMessage(to, content, options);
    
    if (sentMsg?.key?.id) {
      // Log sent message to database
      try {
        await dbQuery(
          `INSERT INTO message_logs (session_id, message_id, remote_jid, from_me, status)
           VALUES ($1, $2, $3, $4, 'SENT')
           ON CONFLICT DO NOTHING`,
          [sessionId, sentMsg.key.id, to, true]
        );
      } catch (err) {
        logger.error(err, 'Failed to insert message_log');
      }
    }
    
    return sentMsg;
  },

  async getBufferFromUrl(url: string): Promise<Buffer> {
    const parsedUrl = await assertSafeExternalUrl(url);
    const response = await fetch(parsedUrl.toString(), {
      signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
      redirect: 'follow',
    });
    if (!response.ok) {
      throw new Error(`Failed to fetch media from URL: ${response.statusText}`);
    }

    const contentLength = Number(response.headers.get('content-length') || 0);
    if (contentLength > MAX_REMOTE_MEDIA_BYTES) {
      throw new Error(`Remote media exceeds maximum size limit (${(MAX_REMOTE_MEDIA_BYTES / 1024 / 1024).toFixed(0)} MB)`);
    }

    if (!response.body) {
      throw new Error('Empty media response body');
    }

    const chunks: Buffer[] = [];
    let totalBytes = 0;
    const reader = response.body.getReader();

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      if (value) {
        totalBytes += value.byteLength;
        if (totalBytes > MAX_REMOTE_MEDIA_BYTES) {
          await reader.cancel();
          throw new Error(`Remote media exceeds maximum size limit (${(MAX_REMOTE_MEDIA_BYTES / 1024 / 1024).toFixed(0)} MB)`);
        }
        chunks.push(Buffer.from(value));
      }
    }

    return Buffer.concat(chunks, totalBytes);
  }
};
