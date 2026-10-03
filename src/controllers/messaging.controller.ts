import { Request, Response } from 'express';
import { messagingService } from '../services/messaging.service';
import { downloadMediaMessage, WAMessage } from '@whiskeysockets/baileys';
import { redis } from '../config/redis';

export const messagingController = {
  async sendText(req: Request, res: Response) {
    const sessionId = req.params.sessionId as string;
    const { to, text, quotedMessageId } = req.body;

    try {
      let options = {};
      if (quotedMessageId) {
        // Construct mock WAMessage for quoting
        options = { quoted: { key: { remoteJid: to, id: quotedMessageId } } };
      }

      const result = await messagingService.sendMessage(sessionId, to, { text }, options);
      res.json({ success: true, data: result });
    } catch (error: any) {
      res.status(500).json({ success: false, error: { message: error.message } });
    }
  },

  async sendMedia(req: Request, res: Response) {
    const sessionId = req.params.sessionId as string;
    const { to, type, url, base64, caption, fileName, mimetype, ptt, quotedMessageId } = req.body;

    try {
      let mediaBuffer: Buffer;
      if (url) {
        mediaBuffer = await messagingService.getBufferFromUrl(url);
      } else {
        // Terima juga data URL ("data:image/png;base64,...."): tanpa dibuang, awalan itu ikut
        // didekode (huruf dan "/" sah di base64) dan filenya rusak.
        mediaBuffer = Buffer.from(base64.replace(/^data:[^,]*;base64,/, ''), 'base64');
      }

      let content: any = {};
      if (type === 'image') content = { image: mediaBuffer, caption, mimetype };
      else if (type === 'video') content = { video: mediaBuffer, caption, mimetype };
      else if (type === 'audio') content = { audio: mediaBuffer, mimetype: mimetype || 'audio/mp4', ptt: !!ptt };
      else if (type === 'document') content = { document: mediaBuffer, caption, mimetype, fileName };
      else if (type === 'sticker') content = { sticker: mediaBuffer };

      let options = {};
      if (quotedMessageId) {
        options = { quoted: { key: { remoteJid: to, id: quotedMessageId } } };
      }

      const result = await messagingService.sendMessage(sessionId, to, content, options);
      res.json({ success: true, data: result });
    } catch (error: any) {
      res.status(500).json({ success: false, error: { message: error.message } });
    }
  },

  async react(req: Request, res: Response) {
    const sessionId = req.params.sessionId as string;
    const { to, messageId, emoji } = req.body;

    try {
      const content = {
        react: { text: emoji, key: { remoteJid: to, id: messageId } }
      };
      const result = await messagingService.sendMessage(sessionId, to, content);
      res.json({ success: true, data: result });
    } catch (error: any) {
      res.status(500).json({ success: false, error: { message: error.message } });
    }
  },

  async revoke(req: Request, res: Response) {
    const sessionId = req.params.sessionId as string;
    const { to, messageId, fromMe } = req.body;

    try {
      const content = {
        delete: { remoteJid: to, id: messageId, fromMe: fromMe ?? true }
      };
      const result = await messagingService.sendMessage(sessionId, to, content);
      res.json({ success: true, data: result });
    } catch (error: any) {
      res.status(500).json({ success: false, error: { message: error.message } });
    }
  },
  
  async sendLocation(req: Request, res: Response) {
    const sessionId = req.params.sessionId as string;
    const { to, latitude, longitude, name, address } = req.body;

    try {
      const content = {
        location: { degreesLatitude: latitude, degreesLongitude: longitude, name, address }
      };
      const result = await messagingService.sendMessage(sessionId, to, content);
      res.json({ success: true, data: result });
    } catch (error: any) {
      res.status(500).json({ success: false, error: { message: error.message } });
    }
  },

  async download(req: Request, res: Response) {
    const sessionId = req.params.sessionId as string;
    const messageId = req.params.messageId as string;

    try {
      const cachedMsgStr = await redis.get(`message:${sessionId}:${messageId}`);
      if (!cachedMsgStr) {
        return res.status(404).json({ success: false, error: { code: 'NOT_FOUND', message: 'Message media not found in cache. Media messages are only cached for 24 hours.' } });
      }

      const msg: WAMessage = JSON.parse(cachedMsgStr);
      const buffer = await downloadMediaMessage(
        msg,
        'buffer',
        {},
        { 
          logger: console as any,
          reuploadRequest: (msg) => new Promise((resolve) => resolve(msg))
        }
      );

      // Guess mimetype
      const msgContent = msg.message?.imageMessage || msg.message?.videoMessage || msg.message?.audioMessage || msg.message?.documentMessage || msg.message?.stickerMessage;
      const mimetype = msgContent?.mimetype || 'application/octet-stream';

      res.setHeader('Content-Type', mimetype);
      res.send(buffer);
    } catch (error: any) {
      res.status(500).json({ success: false, error: { message: error.message } });
    }
  }
};
