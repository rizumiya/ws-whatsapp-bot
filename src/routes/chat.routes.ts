import { Router } from 'express';
import { chatController } from '../controllers/chat.controller';
import { sessionAuthGuard } from '../middlewares/auth.middleware';
import { validate } from '../middlewares/validator.middleware';
import { markReadSchema, presenceSchema } from '../validators/chat.schema';

export const chatRoutes = Router({ mergeParams: true });

chatRoutes.use(sessionAuthGuard);

chatRoutes.post('/read', validate(markReadSchema), chatController.markRead);
chatRoutes.post('/presence', validate(presenceSchema), chatController.updatePresence);
