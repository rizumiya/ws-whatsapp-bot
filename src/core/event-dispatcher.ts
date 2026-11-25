import { sessionRepository } from '../repositories/session.repository';
import { dbQuery } from '../config/database';
import { logger } from '../utils/logger';

// Event berfrekuensi tinggi (seperti indikator mengetik/online) tetap dikirim ke webhook,
// tetapi tidak disimpan ke tabel webhook_logs agar ukuran database PostgreSQL tidak membengkak.
const EPHEMERAL_EVENTS = new Set(['presence.update', 'contacts.update']);

// Pembersihan berkala log lama (> 7 hari) setiap 6 jam agar disk VPS tidak penuh.
const LOG_CLEANUP_INTERVAL_MS = 6 * 60 * 60 * 1000;
setInterval(async () => {
  try {
    await dbQuery(`DELETE FROM webhook_logs WHERE created_at < NOW() - INTERVAL '7 days'`);
    await dbQuery(`DELETE FROM message_logs WHERE created_at < NOW() - INTERVAL '7 days'`);
  } catch (err) {
    logger.warn({ err }, 'Failed to prune old webhook/message logs');
  }
}, LOG_CLEANUP_INTERVAL_MS).unref();

export const dispatchWebhook = async (sessionId: string, eventType: string, payload: any) => {
  try {
    const session = await sessionRepository.get(sessionId);
    if (!session || !session.webhook_url) return;

    const webhookUrl = session.webhook_url;
    
    // Fire and forget, let retry logic handle it in background
    processWebhookWithRetry(sessionId, webhookUrl, eventType, payload, 0);
  } catch (error) {
    logger.error(error, `Failed to dispatch webhook for session ${sessionId}`);
  }
};

const processWebhookWithRetry = async (
  sessionId: string, 
  webhookUrl: string, 
  eventType: string, 
  payload: any, 
  retryCount: number
) => {
  const shouldPersistLog = !EPHEMERAL_EVENTS.has(eventType);

  try {
    const response = await fetch(webhookUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Session-Id': sessionId,
        'X-Event-Type': eventType
      },
      body: JSON.stringify(payload),
      signal: AbortSignal.timeout(15000),
    });

    if (shouldPersistLog) {
      await dbQuery(
        `INSERT INTO webhook_logs (session_id, event_type, payload, response_status, retry_count, status)
         VALUES ($1, $2, $3, $4, $5, $6)`,
        [sessionId, eventType, JSON.stringify(payload), response.status, retryCount, response.ok ? 'DELIVERED' : 'FAILED']
      );
    }

    if (!response.ok && retryCount < 3 && shouldPersistLog) {
      scheduleRetry(sessionId, webhookUrl, eventType, payload, retryCount + 1);
    }
  } catch (error: any) {
    logger.error({ sessionId, webhookUrl, error: error.message }, 'Webhook HTTP request failed');
    
    if (shouldPersistLog) {
      await dbQuery(
        `INSERT INTO webhook_logs (session_id, event_type, payload, response_status, retry_count, status)
         VALUES ($1, $2, $3, null, $4, 'FAILED')`,
        [sessionId, eventType, JSON.stringify(payload), retryCount]
      ).catch(() => undefined);

      if (retryCount < 3) {
        scheduleRetry(sessionId, webhookUrl, eventType, payload, retryCount + 1);
      }
    }
  }
};

const scheduleRetry = (sessionId: string, webhookUrl: string, eventType: string, payload: any, retryCount: number) => {
  // Exponential backoff: 5s, 15s, 45s
  const delay = Math.pow(3, retryCount - 1) * 5000;
  setTimeout(() => {
    processWebhookWithRetry(sessionId, webhookUrl, eventType, payload, retryCount);
  }, delay);
};
