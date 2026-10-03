import { Request, Response, NextFunction } from 'express';
import { ZodError } from 'zod';
import { logger } from '../utils/logger';

export const errorHandler = (
  err: any,
  req: Request,
  res: Response,
  next: NextFunction
) => {
  if (err instanceof ZodError) {
    return res.status(400).json({
      success: false,
      error: {
        code: 'VALIDATION_ERROR',
        message: 'Invalid request payload',
        details: err.issues,
      },
    });
  }

  // Dari body-parser: badan permintaan melebihi batas (lihat MEDIA_BODY_LIMIT di env).
  if (err?.type === 'entity.too.large') {
    const mb = (n: number) => `${(n / 1024 / 1024).toFixed(1)} MB`;
    return res.status(413).json({
      success: false,
      error: {
        code: 'PAYLOAD_TOO_LARGE',
        message: `Request body too large (${mb(err.length)}, limit ${mb(err.limit)}). Send large media via "url" instead of "base64".`,
      },
    });
  }

  logger.error(err, 'Unhandled Error');

  const statusCode = err.statusCode || 500;
  const message = err.message || 'Internal Server Error';

  res.status(statusCode).json({
    success: false,
    error: {
      code: 'SERVER_ERROR',
      message,
    },
  });
};
