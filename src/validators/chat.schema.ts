import { z } from 'zod';
import { jidSchema } from './messaging.schema';

export const markReadSchema = z.object({
  to: jidSchema,
  messageId: z.string(),
});

export const presenceSchema = z.object({
  to: jidSchema,
  presence: z.enum(['composing', 'recording', 'paused']),
});
