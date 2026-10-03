import { z } from 'zod';
import dotenv from 'dotenv';

dotenv.config();

const envSchema = z.object({
  PORT: z.string().default('3000'),
  NODE_ENV: z.string().default('development'),
  MASTER_API_KEY: z.string().min(1, 'MASTER_API_KEY is required for admin endpoints'),
  
  DB_HOST: z.string().default('localhost'),
  DB_PORT: z.string().default('5432'),
  DB_USER: z.string().default('root'),
  DB_PASSWORD: z.string().default('rootpassword'),
  DB_NAME: z.string().default('whatsapp_bot'),
  
  REDIS_HOST: z.string().default('localhost'),
  REDIS_PORT: z.string().default('6379'),
  REDIS_PASSWORD: z.string().optional(),
  
  LOG_LEVEL: z.string().default('info'),

  // Batas badan permintaan POST .../messages/media (media base64 membengkak ~4/3).
  // 25mb = file sekitar 18 MB, sudah di atas batas foto/video WhatsApp (16 MB).
  MEDIA_BODY_LIMIT: z.string().default('25mb'),
});

const _env = envSchema.safeParse(process.env);

if (!_env.success) {
  console.error('❌ Invalid environment variables:', _env.error.format());
  process.exit(1);
}

export const env = _env.data;
