CREATE TABLE users(
    id SERIAL PRIMARY KEY,
    balance bigint NOT NULL CHECK (balance >= 0) DEFAULT (0), -- Amount stored in millisatoshis
    email VARCHAR(256) NOT NULL,
    hashed_password BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
COMMENT ON COLUMN users.balance is 'Balance stored in millisatoshis.';

