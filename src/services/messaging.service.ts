import { AnyMessageContent, delay, WASocket } from '@whiskeysockets/baileys';
import { dbQuery } from '../config/database';
import { sessionManager } from '../core/session-manager';
import { logger } from '../utils/logger';

export const messagingService = {
  async sendMessage(
    sessionId: string,
    to: string,
    content: AnyMessageContent,
    options?: any
  ) {
    const sock = sessionManager.getSession(sessionId);
    if (!sock) {
      throw new Error(`Session ${sessionId} is not connected`);
    }

    // Optional: Add small delay for human-like presence
    await sock.presenceSubscribe(to);
    await delay(300);
    await sock.sendPresenceUpdate('composing', to);
    await delay(600);
    await sock.sendPresenceUpdate('paused', to);

    const sentMsg = await sock.sendMessage(to, content, options);
    
    if (sentMsg?.key?.id) {
      // Log sent message to database
      try {
        await dbQuery(
          `INSERT INTO message_logs (session_id, message_id, remote_jid, from_me, status)
           VALUES ($1, $2, $3, $4, 'SENT')`,
          [sessionId, sentMsg.key.id, to, true]
        );
      } catch (err) {
        logger.error(err, 'Failed to insert message_log');
      }
    }
    
    return sentMsg;
  },

  async getBufferFromUrl(url: string): Promise<Buffer> {
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`Failed to fetch media from URL: ${response.statusText}`);
    }
    const arrayBuffer = await response.arrayBuffer();
    return Buffer.from(arrayBuffer);
  }
};
