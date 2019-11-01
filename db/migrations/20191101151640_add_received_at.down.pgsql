ALTER TABLE transactions
    DROP COLUMN received_tx_at
    DROP CONSTRAINT transactions_check_received_tx_at;