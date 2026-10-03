-- 3. Log pengiriman webhook keluar (Audit & Dead Letter Queue)
CREATE TABLE IF NOT EXISTS webhook_logs (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(100) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    response_status INT NULL,
    retry_count INT DEFAULT 0,
    status VARCHAR(20) DEFAULT 'PENDING', -- PENDING, DELIVERED, FAILED
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webhook_logs_session_id ON webhook_logs(session_id);
