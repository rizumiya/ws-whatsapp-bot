import { sessionRepository } from '../repositories/session.repository';
import { dbQuery } from '../config/database';
import { logger } from '../utils/logger';

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
  try {
    const response = await fetch(webhookUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Session-Id': sessionId,
        'X-Event-Type': eventType
      },
      body: JSON.stringify(payload)
    });

    await dbQuery(
      `INSERT INTO webhook_logs (session_id, event_type, payload, response_status, retry_count, status)
       VALUES ($1, $2, $3, $4, $5, $6)`,
      [sessionId, eventType, JSON.stringify(payload), response.status, retryCount, response.ok ? 'DELIVERED' : 'FAILED']
    );

    if (!response.ok && retryCount < 3) {
      scheduleRetry(sessionId, webhookUrl, eventType, payload, retryCount + 1);
    }
  } catch (error: any) {
    logger.error({ sessionId, webhookUrl, error: error.message }, 'Webhook HTTP request failed');
    
    await dbQuery(
      `INSERT INTO webhook_logs (session_id, event_type, payload, response_status, retry_count, status)
       VALUES ($1, $2, $3, null, $4, 'FAILED')`,
      [sessionId, eventType, JSON.stringify(payload), retryCount]
    );

    if (retryCount < 3) {
      scheduleRetry(sessionId, webhookUrl, eventType, payload, retryCount + 1);
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
