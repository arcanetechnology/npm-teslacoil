DROP TYPE IF EXISTS direction;
DROP TYPE IF EXISTS status;
CREATE TYPE direction AS ENUM ('INBOUND', 'OUTBOUND');
CREATE TYPE status AS ENUM ('SUCCEEDED', 'FAILED', 'IN-FLIGHT');
CREATE TABLE offchaintx(
    id SERIAL PRIMARY KEY,
    user_id integer REFERENCES users(id),
    payment_request TEXT NOT NULL,
    amount_sat bigint NOT NULL CHECK (amount_sat >= 0) DEFAULT (0), -- Amount stored in satoshis
    amount_msat bigint NOT NULL CHECK (amount_msat >= 0) DEFAULT (0), -- Amount stored in millisatoshis
    CHECK (amount_msat = (amount_sat * 1000)),
    settled_at TIMESTAMPTZ,
    status status,
    description VARCHAR(256),
    direction direction NOT NULL,
    preimage VARCHAR(256),
    hashed_preimage VARCHAR(256),
    callback_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
COMMENT ON COLUMN offchaintx.amount_msat is 'Amount stored in millisatoshis.';
COMMENT ON COLUMN offchaintx.amount_sat is 'Amount in satoshis, should always be 1/1000 of amount_msat.';
