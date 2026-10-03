import { Request, Response } from 'express';
import { sessionManager } from '../core/session-manager';

const getSock = (sessionId: string) => {
  const sock = sessionManager.getSession(sessionId);
  if (!sock) throw new Error('Session not connected');
  return sock;
};

export const chatController = {
  async markRead(req: Request, res: Response) {
    try {
      const sock = getSock(req.params.sessionId as string);
      const { to, messageId } = req.body;
      const key = { remoteJid: to, id: messageId, fromMe: false };
      await sock.readMessages([key]);
      res.json({ success: true, message: 'Message marked as read' });
    } catch (error: any) { res.status(500).json({ success: false, error: { message: error.message } }); }
  },

  async updatePresence(req: Request, res: Response) {
    try {
      const sock = getSock(req.params.sessionId as string);
      const { to, presence } = req.body;
      await sock.sendPresenceUpdate(presence, to);
      res.json({ success: true, message: 'Presence updated' });
    } catch (error: any) { res.status(500).json({ success: false, error: { message: error.message } }); }
  }
};
