import { Router } from 'express';
import { sessionController } from '../controllers/session.controller';
import { masterAuthGuard, sessionAuthGuard } from '../middlewares/auth.middleware';
import { validate } from '../middlewares/validator.middleware';
import { createSessionSchema, requestPairingCodeSchema } from '../validators/session.schema';

export const sessionRoutes = Router();

// Master API Key required
sessionRoutes.get('/', masterAuthGuard, sessionController.list);
sessionRoutes.post('/', masterAuthGuard, validate(createSessionSchema), sessionController.create);

// Master API Key OR Session API Key required
sessionRoutes.get('/:sessionId/status', sessionAuthGuard, sessionController.status);
sessionRoutes.get('/:sessionId/qr', sessionAuthGuard, sessionController.getQr);
sessionRoutes.post('/:sessionId/pairing-code', sessionAuthGuard, validate(requestPairingCodeSchema), sessionController.requestPairingCode);
sessionRoutes.post('/:sessionId/restart', sessionAuthGuard, sessionController.restart);
sessionRoutes.delete('/:sessionId/logout', sessionAuthGuard, sessionController.logout);
