ALTER TABLE transactions
    ADD COLUMN received_tx_at TIMESTAMPTZ;

-- the last update is probably the timestamp when a tx was received
UPDATE transactions
SET received_tx_at = updated_at
WHERE received_tx_at IS NULL
  AND txid IS NOT NULL;

-- updated_at can be NULL for some transactions
UPDATE transactions
SET received_tx_at = created_at
WHERE received_tx_at IS NULL
  AND txid IS NOT NULL;

ALTER TABLE transactions
    ADD CONSTRAINT transactions_check_received_tx_at CHECK (
            (txid IS NULL AND
             received_tx_at IS NULL) OR
            (txid IS NOT NULL AND
             received_tx_at IS NOT NULL)
        )

