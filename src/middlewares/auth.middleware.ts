import { Request, Response, NextFunction } from 'express';
import { env } from '../config/env';
import { sessionRepository } from '../repositories/session.repository';
import crypto from 'crypto';

export const hashApiKey = (apiKey: string): string => {
  return crypto.createHash('sha256').update(apiKey).digest('hex');
};

const safeCompare = (a: string, b: string): boolean => {
  const bufA = Buffer.from(a);
  const bufB = Buffer.from(b);
  if (bufA.length !== bufB.length) return false;
  return crypto.timingSafeEqual(bufA, bufB);
};

export const masterAuthGuard = (req: Request, res: Response, next: NextFunction) => {
  const apiKeyRaw = req.headers['x-api-key'] || req.headers['authorization'];
  const apiKey = Array.isArray(apiKeyRaw) ? apiKeyRaw[0] : apiKeyRaw?.replace('Bearer ', '');
  
  if (!apiKey || !safeCompare(apiKey, env.MASTER_API_KEY)) {
    return res.status(401).json({ success: false, error: { code: 'UNAUTHORIZED', message: 'Invalid or missing Master API Key' } });
  }
  next();
};

export const sessionAuthGuard = async (req: Request, res: Response, next: NextFunction) => {
  const apiKeyRaw = req.headers['x-api-key'] || req.headers['authorization'];
  const apiKey = Array.isArray(apiKeyRaw) ? apiKeyRaw[0] : apiKeyRaw?.replace('Bearer ', '');
  const sessionId = req.params.sessionId as string;
  
  if (!sessionId) {
    return res.status(400).json({ success: false, error: { code: 'BAD_REQUEST', message: 'Missing sessionId parameter' } });
  }

  const session = await sessionRepository.get(sessionId);
  if (!session) {
    return res.status(404).json({ success: false, error: { code: 'NOT_FOUND', message: `Session ${sessionId} not found` } });
  }

  if (!apiKey || !safeCompare(apiKey, env.MASTER_API_KEY)) {
    if (!apiKey) {
      return res.status(401).json({ success: false, error: { code: 'UNAUTHORIZED', message: 'Missing API Key' } });
    }
    const hashedKey = hashApiKey(apiKey);
    if (!session.api_key || !safeCompare(hashedKey, session.api_key)) {
      return res.status(401).json({ success: false, error: { code: 'UNAUTHORIZED', message: 'Invalid API Key for this session' } });
    }
  }
  
  (req as any).sessionData = session;
  next();
};
