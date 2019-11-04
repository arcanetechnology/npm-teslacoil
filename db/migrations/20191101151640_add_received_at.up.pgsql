ALTER TABLE transactions
    ADD COLUMN received_tx_at TIMESTAMPTZ;

ALTER TABLE transactions
    ADD CONSTRAINT transactions_check_received_tx_at CHECK (
        (txid IS NULL AND
        received_tx_at IS NULL) OR
        (txid IS NOT NULL AND
         received_tx_at IS NOT NULL)
    )

