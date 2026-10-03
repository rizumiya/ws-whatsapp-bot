import { z } from 'zod';

export const createSessionSchema = z.object({
  sessionId: z.string().min(3).max(64).regex(/^[a-zA-Z0-9_-]+$/, 'Only alphanumeric, dash and underscore allowed'),
  webhookUrl: z.string().url().optional(),
});

export const requestPairingCodeSchema = z.object({
  phoneNumber: z.string().min(8).regex(/^[0-9]+$/, 'Only numbers allowed without + or spaces. Example: 62812345678'),
});
