-- add new fields from offchaintx to transaction
ALTER TABLE transactions
    -- add common fields for both lightning and on-chain transactions
    ADD COLUMN amount_milli_sat bigint,
    ADD COLUMN callback_url       TEXT,
    ADD COLUMN expiry             bigint,
    ADD COLUMN customer_order_id  VARCHAR(256),
    ADD COLUMN settled_at TIMESTAMPTZ,
    ALTER COLUMN address DROP NOT NULL,

    -- add fields for lightning transactions
    ADD COLUMN memo               VARCHAR(256),
    ADD COLUMN payment_request    TEXT,
    ADD COLUMN preimage           bytea,
    ADD COLUMN hashed_preimage    bytea,
    ADD COLUMN invoice_status     status,

    -- alter fields for on-chain transactions
    ADD COLUMN confirmed_at_block int,
    DROP COLUMN confirmed;

-- update all comments
COMMENT ON COLUMN transactions.amount_sat is 'Amount stored in millisatoshis.';
COMMENT ON COLUMN transactions.expiry IS 'expiry in seconds for when this transactions is no longer considered valid. for lightning transactions, this is the invoice expiry';
COMMENT ON COLUMN transactions.memo IS 'memo stored on lightning invoice';
COMMENT ON COLUMN transactions.description IS 'personal/internal description';
COMMENT ON COLUMN transactions.settled_at IS 'the settlement date. for an invoice its when its paid, onchain when it meets user specified confirmations';
COMMENT ON COLUMN transactions.invoice_status IS 'the current status of an invoice, in (SUCCEEDED, IN-FLIGHT, OPEN, FAILED) ';
COMMENT ON COLUMN transactions.confirmed_at_block IS 'the height for when this transaction was confirmed';
