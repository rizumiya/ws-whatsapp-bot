import { Router } from 'express';
import { sessionRoutes } from './session.routes';
import { messagingRoutes } from './messaging.routes';
import { groupRoutes } from './group.routes';
import { chatRoutes } from './chat.routes';

export const routes = Router();

routes.use('/sessions', sessionRoutes);
routes.use('/sessions/:sessionId/messages', messagingRoutes);
routes.use('/sessions/:sessionId/groups', groupRoutes);
routes.use('/sessions/:sessionId/chats', chatRoutes);
