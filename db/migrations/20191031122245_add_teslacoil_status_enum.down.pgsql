-- it is not possible to delete values from an enum, therefore we create a new one
CREATE TYPE status AS ENUM ('SUCCEEDED', 'FAILED', 'IN-FLIGHT', 'OPEN');

ALTER TABLE transactions
    ALTER COLUMN invoice_status TYPE TEXT;

UPDATE transactions
SET invoice_status = 'OPEN'
WHERE invoice_status = 'CREATED';

UPDATE transactions
SET invoice_status = 'IN-FLIGHT'
WHERE invoice_status = 'SENT';

UPDATE transactions
SET invoice_status = 'SUCCEEDED'
WHERE invoice_status = 'COMPLETED';

UPDATE transactions
SET invoice_status = 'FAILED'
WHERE invoice_status = 'FLOPPED';

ALTER TABLE transactions
    ALTER COLUMN invoice_status TYPE status USING invoice_status::text::status;
DROP TYPE IF EXISTS offchain_status;
