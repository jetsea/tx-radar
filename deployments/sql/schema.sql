
CREATE TABLE IF NOT EXISTS transactions (
    hash            VARCHAR(66) PRIMARY KEY,
    from_addr       VARCHAR(42) NOT NULL,
    to_addr         VARCHAR(42),
    value           NUMERIC(78, 0),
    nonce           BIGINT,
    gas_price       NUMERIC(78, 0),
    gas_limit       BIGINT,
    gas_tip_cap     NUMERIC(78, 0),
    gas_fee_cap     NUMERIC(78, 0),
    data            BYTEA,
    chain_id        NUMERIC(78, 0),
    source          VARCHAR(20),
    block_number    BIGINT,
    timestamp       TIMESTAMPTZ NOT NULL,
    tx_type         VARCHAR(50),
    method_id       VARCHAR(10),
    level           VARCHAR(10),
    is_high_value   BOOLEAN DEFAULT FALSE,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);


CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions (timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_from ON transactions (from_addr);
CREATE INDEX IF NOT EXISTS idx_transactions_to ON transactions (to_addr);
--partial index for high value transactions
CREATE INDEX IF NOT EXISTS idx_transactions_level ON transactions (level) WHERE level = 'HIGH';
CREATE INDEX IF NOT EXISTS idx_transactions_tx_type ON transactions (tx_type);
CREATE INDEX IF NOT EXISTS idx_transactions_block ON transactions (block_number);
