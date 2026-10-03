-- 2. Penyimpanan Kredensial Baileys (Multi-Session Key-Value Store)
CREATE TABLE IF NOT EXISTS session_auth_keys (
    session_id VARCHAR(100) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    key_type VARCHAR(50) NOT NULL, -- 'creds', 'pre-key', 'session', 'sender-key', dll.
    key_id VARCHAR(255) NOT NULL,
    value JSONB NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (session_id, key_type, key_id)
);

CREATE INDEX IF NOT EXISTS idx_session_auth_keys_lookup 
ON session_auth_keys (session_id, key_type);
