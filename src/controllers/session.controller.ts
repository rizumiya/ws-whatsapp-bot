import { Request, Response } from 'express';
import crypto from 'crypto';
import QRCode from 'qrcode';
import { sessionManager } from '../core/session-manager';
import { sessionRepository } from '../repositories/session.repository';
import { hashApiKey } from '../middlewares/auth.middleware';
import { redis } from '../config/redis';
import { dbQuery } from '../config/database';
import { logger } from '../utils/logger';

export const sessionController = {
  async list(req: Request, res: Response) {
    const sessions = await sessionRepository.getAll();
    // Mask api keys in response
    const sanitized = sessions.map(s => {
      const { api_key, ...rest } = s;
      return { ...rest, runtime: sessionManager.runtimeState(s.id) };
    });
    res.json({ success: true, data: sanitized });
  },

  async create(req: Request, res: Response) {
    const { webhookUrl } = req.body;
    const sessionId = req.body.sessionId as string;
    
    const existing = await sessionRepository.get(sessionId);
    if (existing) {
      return res.status(400).json({ success: false, error: { code: 'ALREADY_EXISTS', message: 'Session already exists' } });
    }

    const plainApiKey = crypto.randomBytes(32).toString('hex');
    const hashedKey = hashApiKey(plainApiKey);

    await sessionRepository.create(sessionId, hashedKey);
    
    if (webhookUrl) {
      await dbQuery(`UPDATE sessions SET webhook_url = $1 WHERE id = $2`, [webhookUrl, sessionId]);
    }

    // Initialize socket
    sessionManager.createSession(sessionId, hashedKey).catch(console.error);

    res.json({
      success: true,
      data: {
        sessionId,
        apiKey: plainApiKey,
        status: 'STARTING'
      }
    });
  },

  async status(req: Request, res: Response) {
    const sessionId = req.params.sessionId as string;
    const session = await sessionRepository.get(sessionId);
    if (!session) {
      return res.status(404).json({ success: false, error: { code: 'NOT_FOUND', message: 'Session not found' } });
    }
    const { api_key, ...rest } = session;
    res.json({ success: true, data: { ...rest, runtime: sessionManager.runtimeState(sessionId) } });
  },

  async getQr(req: Request, res: Response) {
    const sessionId = req.params.sessionId as string;
    const qr = await redis.get(`session:${sessionId}:qr`);
    if (!qr) {
      return res.status(404).json({ success: false, error: { code: 'NOT_FOUND', message: 'QR Code not available. Session might be already connected or still starting.' } });
    }
    // qrImage: gambar PNG (data URL) siap ditampilkan / di-scan, supaya klien tidak perlu library QR sendiri.
    const qrImage = await QRCode.toDataURL(qr, { margin: 1, width: 320, errorCorrectionLevel: 'M' });
    res.json({ success: true, data: { qr, qrImage } });
  },

  async requestPairingCode(req: Request, res: Response) {
    const sessionId = req.params.sessionId as string;
    const { phoneNumber } = req.body;
    const sock = sessionManager.getSession(sessionId);
    
    if (!sock) {
      return res.status(400).json({ success: false, error: { code: 'NOT_ACTIVE', message: 'Socket is not active. Wait for STARTING status.' } });
    }

    try {
      const code = await sock.requestPairingCode(phoneNumber);
      res.json({ success: true, data: { code } });
    } catch (error: any) {
      res.status(500).json({ success: false, error: { code: 'PAIRING_ERROR', message: error.message } });
    }
  },

  async restart(req: Request, res: Response) {
    const sessionId = req.params.sessionId as string;
    await sessionManager.deleteSession(sessionId, false); 
    // Will use existing hash
    sessionManager.createSession(sessionId, '').catch(err => logger.error(err, `Restart failed for ${sessionId}`));
    res.json({ success: true, message: 'Session restart initiated' });
  },

  async logout(req: Request, res: Response) {
    const sessionId = req.params.sessionId as string;
    await sessionManager.removeSession(sessionId);
    res.json({ success: true, message: 'Session logged out and deleted' });
  }
};
