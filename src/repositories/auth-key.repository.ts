import { dbQuery } from '../config/database';

export const authKeyRepository = {
  async get(sessionId: string, keyType: string, keyId: string): Promise<any | null> {
    const res = await dbQuery(
      `SELECT value FROM session_auth_keys WHERE session_id = $1 AND key_type = $2 AND key_id = $3`,
      [sessionId, keyType, keyId]
    );
    if (res.rows.length > 0) {
      return res.rows[0].value;
    }
    return null;
  },

  async set(sessionId: string, keyType: string, keyId: string, value: any): Promise<void> {
    // We expect the value to already be stringified with BufferJSON if it contains Buffers
    await dbQuery(
      `INSERT INTO session_auth_keys (session_id, key_type, key_id, value)
       VALUES ($1, $2, $3, $4)
       ON CONFLICT (session_id, key_type, key_id) 
       DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP`,
      [sessionId, keyType, keyId, value]
    );
  },

  async del(sessionId: string, keyType: string, keyId: string): Promise<void> {
    await dbQuery(
      `DELETE FROM session_auth_keys WHERE session_id = $1 AND key_type = $2 AND key_id = $3`,
      [sessionId, keyType, keyId]
    );
  },

  async clearAll(sessionId: string): Promise<void> {
    await dbQuery(
      `DELETE FROM session_auth_keys WHERE session_id = $1`,
      [sessionId]
    );
  }
};
