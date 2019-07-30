DROP TYPE IF EXISTS direction;
CREATE TYPE direction AS ENUM ('inbound', 'outbound');
CREATE TABLE transactions(
    id SERIAL PRIMARY KEY,
    user_id integer REFERENCES users(id),
    invoice TEXT NOT NULL,
    amount bigint NOT NULL CHECK (amount >= 0) DEFAULT (0), -- Amount stored in millisatoshis
    settled_at TIMESTAMPTZ,
    status VARCHAR(256),
    description VARCHAR(256),
    direction direction NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
COMMENT ON COLUMN transactions.amount is 'Amount stored in millisatoshis.';
