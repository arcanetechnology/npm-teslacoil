-- it is not possible to delete values from an enum, therefore we
-- 1) set data type to text, as ALTER TYPE ENUM cannot be run in a transaction...
-- 2) transfer all data
-- 3) set the new type and delete the old type

-- 1)
ALTER TABLE transactions
    ALTER COLUMN invoice_status TYPE TEXT;

-- 2)
UPDATE transactions
SET invoice_status = 'CREATED'
WHERE invoice_status = 'OPEN';

UPDATE transactions
SET invoice_status = 'SENT'
WHERE invoice_status = 'IN-FLIGHT';

UPDATE transactions
SET invoice_status = 'COMPLETED'
WHERE invoice_status = 'SUCCEEDED';

UPDATE transactions
SET invoice_status = 'FLOPPED'
WHERE invoice_status = 'FAILED';

-- 3)
DROP TYPE IF EXISTS offchain_status;
CREATE TYPE offchain_status AS ENUM ('CREATED', 'SENT', 'COMPLETED', 'FLOPPED');
ALTER TABLE transactions
    ALTER COLUMN invoice_status TYPE offchain_status USING invoice_status::text::offchain_status;
DROP TYPE status;
