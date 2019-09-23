DROP TYPE IF EXISTS transaction_status;
CREATE TYPE transaction_status AS ENUM ('UNCONFIRMED', 'CONFIRMED');
CREATE TABLE transactions(
    id SERIAL PRIMARY KEY,
    user_id integer REFERENCES users(id),
    address varchar(128) NOT NULL,
    txid varchar(256),
    amount_sat bigint NOT NULL CHECK (amount_sat >= 0), -- Amount stored in satoshis
    description TEXT,
    status transaction_status,
    confirmed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
COMMENT ON COLUMN transactions.amount_sat is 'Amount in satoshis';
