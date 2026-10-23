-- Migration: Smart Email Mutation Sync & Auto-Balance Schema Changes
-- Dialect: SQLite / PostgreSQL compatible

-- 1. Alter pocket_pods table
ALTER TABLE pocket_pods ADD COLUMN provider_code VARCHAR(50);
ALTER TABLE pocket_pods ADD COLUMN account_number_suffix VARCHAR(10);
CREATE INDEX IF NOT EXISTS idx_pocket_pods_provider ON pocket_pods(provider_code);

-- 2. Alter transactions table
ALTER TABLE transactions ADD COLUMN source_type VARCHAR(20) DEFAULT 'MANUAL';
ALTER TABLE transactions ADD COLUMN external_ref_id VARCHAR(100);
ALTER TABLE transactions ADD COLUMN sync_status VARCHAR(20) DEFAULT 'VERIFIED';
ALTER TABLE transactions ADD COLUMN admin_fee DECIMAL(15,2) DEFAULT 0;
ALTER TABLE transactions ADD COLUMN direction VARCHAR(20) DEFAULT 'OUT';
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_external_ref ON transactions(external_ref_id);

-- 3. Create email_mutation_logs table
CREATE TABLE IF NOT EXISTS email_mutation_logs (
    id VARCHAR(36) PRIMARY KEY,
    couple_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    message_id VARCHAR(255) NOT NULL UNIQUE,
    sender_email VARCHAR(150) NOT NULL,
    clean_body_text TEXT NOT NULL,
    llm_raw_response TEXT,
    status VARCHAR(30) NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_email_mutation_logs_couple ON email_mutation_logs(couple_id);
CREATE INDEX IF NOT EXISTS idx_email_mutation_logs_msg_id ON email_mutation_logs(message_id);
