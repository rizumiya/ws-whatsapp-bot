import { z } from 'zod';
import { jidSchema } from './messaging.schema';

export const createGroupSchema = z.object({
  subject: z.string().min(1).max(100),
  participants: z.array(jidSchema).min(1),
});

export const groupActionSchema = z.object({
  participants: z.array(jidSchema).min(1),
});

export const updateGroupSubjectSchema = z.object({
  subject: z.string().min(1).max(100),
});

export const updateGroupDescriptionSchema = z.object({
  description: z.string(),
});
