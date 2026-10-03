import express, { Express } from 'express';
import path from 'path';
import cors from 'cors';
import helmet from 'helmet';
import { errorHandler } from './middlewares/error-handler';
import { env } from './config/env';
import { logger } from './utils/logger';

export const app: Express = express();

// Middlewares
app.use(helmet());
app.use(cors());
// Batas bawaan (100kb) untuk semua endpoint, KECUALI kirim media: badannya baru dibaca di
// messaging.routes.ts sesudah kunci API diperiksa, supaya klien tanpa kunci tidak bisa
// membuat server menampung puluhan MB per permintaan.
const jsonParser = express.json();
const isMediaUpload = (req: express.Request) =>
  req.method === 'POST' && /^\/api\/v1\/sessions\/[^/]+\/messages\/media\/?$/.test(req.path);
app.use((req, res, next) => (isMediaUpload(req) ? next() : jsonParser(req, res, next)));
app.use(express.urlencoded({ extended: true }));

// Halaman kelola nomor (public/index.html di "/"). Dipasang sebelum pencatat permintaan
// supaya berkas statis tidak memenuhi log; semua datanya tetap lewat API berkunci di bawah.
app.use(express.static(path.join(__dirname, '..', 'public')));

// Request Logger
app.use((req, res, next) => {
  logger.info({ method: req.method, url: req.url }, 'Incoming request');
  next();
});

// Health check endpoint
app.get('/health', (req, res) => {
  res.status(200).json({ success: true, message: 'Server is healthy' });
});

import { routes } from './routes';
app.use('/api/v1', routes);

// Global Error Handler
app.use(errorHandler);
