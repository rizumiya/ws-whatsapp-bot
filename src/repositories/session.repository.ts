import { dbQuery } from '../config/database';

export const sessionRepository = {
  async create(sessionId: string, apiKey: string): Promise<void> {
    await dbQuery(
      `INSERT INTO sessions (id, status, api_key) VALUES ($1, 'STARTING', $2)`,
      [sessionId, apiKey]
    );
  },
  
  async updateStatus(sessionId: string, status: string, phoneNumber?: string): Promise<void> {
    if (phoneNumber) {
      await dbQuery(
        `UPDATE sessions SET status = $1, phone_number = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`,
        [status, phoneNumber, sessionId]
      );
    } else {
      await dbQuery(
        `UPDATE sessions SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
        [status, sessionId]
      );
    }
  },
  
  async get(sessionId: string): Promise<any | null> {
    const res = await dbQuery(`SELECT * FROM sessions WHERE id = $1`, [sessionId]);
    return res.rows[0] || null;
  },

  async delete(sessionId: string): Promise<void> {
    await dbQuery(`DELETE FROM sessions WHERE id = $1`, [sessionId]);
  },

  async getAll(): Promise<any[]> {
    const res = await dbQuery(`SELECT * FROM sessions`);
    return res.rows;
  }
};
