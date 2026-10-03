import { Request, Response } from 'express';
import { sessionManager } from '../core/session-manager';

const getSock = (sessionId: string) => {
  const sock = sessionManager.getSession(sessionId);
  if (!sock) throw new Error('Session not connected');
  return sock;
};

export const groupController = {
  async create(req: Request, res: Response) {
    try {
      const sock = getSock(req.params.sessionId as string);
      const { subject, participants } = req.body;
      const group = await sock.groupCreate(subject, participants);
      res.json({ success: true, data: group });
    } catch (error: any) { res.status(500).json({ success: false, error: { message: error.message } }); }
  },

  async getMetadata(req: Request, res: Response) {
    try {
      const sock = getSock(req.params.sessionId as string);
      const group = await sock.groupMetadata((req.params.jid as string));
      res.json({ success: true, data: group });
    } catch (error: any) { res.status(500).json({ success: false, error: { message: error.message } }); }
  },

  async addParticipants(req: Request, res: Response) {
    try {
      const sock = getSock(req.params.sessionId as string);
      const { participants } = req.body;
      const result = await sock.groupParticipantsUpdate((req.params.jid as string), participants, 'add');
      res.json({ success: true, data: result });
    } catch (error: any) { res.status(500).json({ success: false, error: { message: error.message } }); }
  },

  async removeParticipants(req: Request, res: Response) {
    try {
      const sock = getSock(req.params.sessionId as string);
      const { participants } = req.body;
      const result = await sock.groupParticipantsUpdate((req.params.jid as string), participants, 'remove');
      res.json({ success: true, data: result });
    } catch (error: any) { res.status(500).json({ success: false, error: { message: error.message } }); }
  },

  async promoteParticipants(req: Request, res: Response) {
    try {
      const sock = getSock(req.params.sessionId as string);
      const { participants } = req.body;
      const result = await sock.groupParticipantsUpdate((req.params.jid as string), participants, 'promote');
      res.json({ success: true, data: result });
    } catch (error: any) { res.status(500).json({ success: false, error: { message: error.message } }); }
  },

  async demoteParticipants(req: Request, res: Response) {
    try {
      const sock = getSock(req.params.sessionId as string);
      const { participants } = req.body;
      const result = await sock.groupParticipantsUpdate((req.params.jid as string), participants, 'demote');
      res.json({ success: true, data: result });
    } catch (error: any) { res.status(500).json({ success: false, error: { message: error.message } }); }
  },

  async updateSubject(req: Request, res: Response) {
    try {
      const sock = getSock(req.params.sessionId as string);
      await sock.groupUpdateSubject((req.params.jid as string), req.body.subject);
      res.json({ success: true, message: 'Subject updated' });
    } catch (error: any) { res.status(500).json({ success: false, error: { message: error.message } }); }
  },

  async updateDescription(req: Request, res: Response) {
    try {
      const sock = getSock(req.params.sessionId as string);
      await sock.groupUpdateDescription((req.params.jid as string), req.body.description);
      res.json({ success: true, message: 'Description updated' });
    } catch (error: any) { res.status(500).json({ success: false, error: { message: error.message } }); }
  },

  async getInviteCode(req: Request, res: Response) {
    try {
      const sock = getSock(req.params.sessionId as string);
      const code = await sock.groupInviteCode((req.params.jid as string));
      res.json({ success: true, data: { code, url: `https://chat.whatsapp.com/${code}` } });
    } catch (error: any) { res.status(500).json({ success: false, error: { message: error.message } }); }
  }
};
