-- it is not possible to delete values from an enum, therefore we create a new one
ALTER TYPE status RENAME TO status_old;
-- the type might already exist, we try to drop it
DROP TYPE IF EXISTS offchain_status;
CREATE TYPE offchain_status AS ENUM ('CREATED', 'SENT', 'CONFIRMED', 'FLOPPED');
ALTER TABLE transactions
    ALTER COLUMN invoice_status TYPE offchain_status USING invoice_status::text::offchain_status;
DROP TYPE status_old;
