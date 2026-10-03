import { z } from 'zod';

export const jidSchema = z.string().regex(/^[0-9]+(@s\.whatsapp\.net|@g\.us)$/, 'Invalid JID format. Must end with @s.whatsapp.net or @g.us');

export const sendTextSchema = z.object({
  to: jidSchema,
  text: z.string().min(1),
  quotedMessageId: z.string().optional(),
});

export const sendMediaSchema = z.object({
  to: jidSchema,
  type: z.enum(['image', 'video', 'audio', 'document', 'sticker']),
  url: z.string().url().optional(),
  base64: z.string().optional(),
  caption: z.string().optional(),
  fileName: z.string().optional(),
  mimetype: z.string().optional(),
  ptt: z.boolean().optional(),
  quotedMessageId: z.string().optional(),
}).refine(data => data.url || data.base64, {
  message: "Either 'url' or 'base64' must be provided",
  path: ['url']
});

export const reactMessageSchema = z.object({
  to: jidSchema,
  messageId: z.string(),
  emoji: z.string().max(10), // empty string means remove reaction
});

export const revokeMessageSchema = z.object({
  to: jidSchema,
  messageId: z.string(),
  fromMe: z.boolean().default(true),
});

export const sendLocationSchema = z.object({
  to: jidSchema,
  latitude: z.number(),
  longitude: z.number(),
  name: z.string().optional(),
  address: z.string().optional(),
});
