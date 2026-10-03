-- 4. Log riwayat pesan ringkas (untuk delete/revoke, quote, read receipt)
CREATE TABLE IF NOT EXISTS message_logs (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(100) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    message_id VARCHAR(255) NOT NULL,
    remote_jid VARCHAR(255) NOT NULL,
    from_me BOOLEAN NOT NULL,
    status VARCHAR(20) DEFAULT 'PENDING', -- PENDING, SENT, DELIVERED, READ
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_message_logs UNIQUE (session_id, message_id, remote_jid, from_me)
);

CREATE INDEX IF NOT EXISTS idx_message_logs_lookup ON message_logs(session_id, message_id);
