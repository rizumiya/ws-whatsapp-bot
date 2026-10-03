import {
  initAuthCreds,
  BufferJSON,
  AuthenticationState,
  AuthenticationCreds,
  SignalDataTypeMap
} from '@whiskeysockets/baileys';
import { authKeyRepository } from '../repositories/auth-key.repository';
import { redis } from '../config/redis';

const CACHE_TTL = 86400; // 24 hours

export const usePostgresAuthState = async (
  sessionId: string
): Promise<{ state: AuthenticationState; saveCreds: () => Promise<void>; clearState: () => Promise<void> }> => {
  const writeData = async (data: any, keyType: string, keyId: string) => {
    const serialized = JSON.stringify(data, BufferJSON.replacer);
    // Write to PostgreSQL
    await authKeyRepository.set(sessionId, keyType, keyId, serialized);
    // Write to Redis
    const cacheKey = `auth:${sessionId}:${keyType}:${keyId}`;
    await redis.set(cacheKey, serialized, 'EX', CACHE_TTL);
  };

  const readData = async (keyType: string, keyId: string) => {
    const cacheKey = `auth:${sessionId}:${keyType}:${keyId}`;
    
    // Try Redis
    let dataStr = await redis.get(cacheKey);
    
    // Try PostgreSQL
    if (!dataStr) {
      dataStr = await authKeyRepository.get(sessionId, keyType, keyId);
      if (dataStr) {
        // Update cache
        await redis.set(cacheKey, dataStr, 'EX', CACHE_TTL);
      }
    }

    if (dataStr) {
      return JSON.parse(dataStr, BufferJSON.reviver);
    }
    return null;
  };

  const removeData = async (keyType: string, keyId: string) => {
    await authKeyRepository.del(sessionId, keyType, keyId);
    const cacheKey = `auth:${sessionId}:${keyType}:${keyId}`;
    await redis.del(cacheKey);
  };

  const clearState = async () => {
    await authKeyRepository.clearAll(sessionId);
    // Clear all redis keys for this session
    const keys = await redis.keys(`auth:${sessionId}:*`);
    if (keys.length > 0) {
      await redis.del(...keys);
    }
  };

  // 1. Init Creds
  let creds: AuthenticationCreds;
  const existingCreds = await readData('creds', 'creds');
  if (existingCreds) {
    creds = existingCreds;
  } else {
    creds = initAuthCreds();
    await writeData(creds, 'creds', 'creds');
  }

  const saveCreds = async () => {
    await writeData(creds, 'creds', 'creds');
  };

  return {
    state: {
      creds,
      keys: {
        get: async (type, ids) => {
          const data: { [id: string]: SignalDataTypeMap[typeof type] } = {};
          await Promise.all(
            ids.map(async (id) => {
              let value = await readData(type, id);
              if (type === 'app-state-sync-key' && value) {
                value = { ...value, syncKey: Buffer.from(value.syncKey, 'base64') };
              }
              data[id] = value;
            })
          );
          return data;
        },
        set: async (data) => {
          const tasks: Promise<void>[] = [];
          for (const category in data) {
            for (const id in data[category as keyof SignalDataTypeMap]) {
              const value = data[category as keyof SignalDataTypeMap]?.[id];
              const keyType = category;
              if (value) {
                tasks.push(writeData(value, keyType, id));
              } else {
                tasks.push(removeData(keyType, id));
              }
            }
          }
          await Promise.all(tasks);
        },
      }
    },
    saveCreds,
    clearState,
  };
};
