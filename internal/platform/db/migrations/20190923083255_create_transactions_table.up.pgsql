DROP TYPE IF EXISTS transaction_status;
CREATE TABLE transactions(
    id SERIAL PRIMARY KEY,
    user_id integer REFERENCES users(id),
    address varchar(128) NOT NULL,
    txid varchar(256) CONSTRAINT txid_length CHECK ( length(txid) = 64 ),
    vout integer,
    UNIQUE (txid, vout),
    CONSTRAINT txid_or_vout_cant_exist_alone CHECK ( (vout IS NOT NULL AND txid IS NOT NULL) OR (vout IS NULL AND txid IS NULL) ),
    direction direction NOT NULL,
    amount_sat bigint CHECK (amount_sat >= 0), -- Amount stored in satoshis
    description TEXT,
    confirmed boolean,
    confirmed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
COMMENT ON COLUMN transactions.amount_sat is 'Amount in satoshis';
