import { app } from './app';
import { env } from './config/env';
import { logger } from './utils/logger';
import { pool } from './config/database';
import { redis } from './config/redis';

const startServer = async () => {
  try {
    // Wait for DB and Redis connection verification
    await pool.query('SELECT 1');
    logger.info('✅ PostgreSQL connected');

    await redis.ping();
    logger.info('✅ Redis connected');

    // Restore sessions from DB
    const { sessionManager } = await import('./core/session-manager');
    await sessionManager.restoreSessions();

    app.listen(env.PORT, () => {
      logger.info(`🚀 Server is running on port ${env.PORT}`);
    });

  } catch (error) {
    logger.error(error, '❌ Failed to start server');
    process.exit(1);
  }
};

// Graceful shutdown handling
const shutdown = async (signal: string) => {
  logger.info(`Received ${signal}. Shutting down gracefully...`);
  try {
    await pool.end();
    await redis.quit();
    logger.info('Closed database and redis connections');
    process.exit(0);
  } catch (error) {
    logger.error(error, 'Error during graceful shutdown');
    process.exit(1);
  }
};

// Handler event Baileys banyak yang async (tulis DB/Redis). Satu kegagalan di sana tidak boleh
// mematikan seluruh proses beserta semua nomor lain yang tersambung.
process.on('unhandledRejection', (reason) => {
  logger.error({ err: reason }, 'Unhandled promise rejection');
});

process.on('SIGTERM', () => shutdown('SIGTERM'));
process.on('SIGINT', () => shutdown('SIGINT'));

startServer();
