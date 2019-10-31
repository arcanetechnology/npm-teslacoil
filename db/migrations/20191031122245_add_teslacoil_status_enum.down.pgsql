-- it is not possible to delete values from an enum, therefore we create a new one
CREATE TYPE status AS ENUM ('SUCCEEDED', 'FAILED', 'IN-FLIGHT');
ALTER TABLE transactions
    ALTER COLUMN invoice_status TYPE status USING invoice_status::text::status;
DROP TYPE IF EXISTS offchain_status;
