import express, { Router } from 'express';
import { env } from '../config/env';
import { messagingController } from '../controllers/messaging.controller';
import { sessionAuthGuard } from '../middlewares/auth.middleware';
import { validate } from '../middlewares/validator.middleware';
import { 
  sendTextSchema, 
  sendMediaSchema, 
  reactMessageSchema, 
  revokeMessageSchema, 
  sendLocationSchema 
} from '../validators/messaging.schema';

export const messagingRoutes = Router({ mergeParams: true });

// All these routes are prefixed with /sessions/:sessionId/messages
messagingRoutes.use(sessionAuthGuard);

messagingRoutes.post('/text', validate(sendTextSchema), messagingController.sendText);
// Badan media dibaca di sini (sesudah sessionAuthGuard), bukan di app.ts. Lihat MEDIA_BODY_LIMIT.
messagingRoutes.post('/media', express.json({ limit: env.MEDIA_BODY_LIMIT }), validate(sendMediaSchema), messagingController.sendMedia);
messagingRoutes.post('/react', validate(reactMessageSchema), messagingController.react);
messagingRoutes.post('/revoke', validate(revokeMessageSchema), messagingController.revoke);
messagingRoutes.post('/location', validate(sendLocationSchema), messagingController.sendLocation);
messagingRoutes.get('/:messageId/download', messagingController.download);
