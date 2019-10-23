-- add all new constraints
ALTER TABLE transactions
    -- remove all constraints
    DROP CONSTRAINT txid_length,
    DROP CONSTRAINT transactions_txid_vout_key,
    DROP CONSTRAINT txid_or_vout_cant_exist_alone,
    DROP CONSTRAINT transactions_amount_sat_check,

    ADD CONSTRAINT transactions_amount_sat_must_be_greater_than_0 CHECK (amount_sat >= 0),

    ADD CONSTRAINT transactions_txid_and_vout_must_be_unique UNIQUE (txid, vout),

    ADD CONSTRAINT transactions_txid_length CHECK ( length(txid) = 64 ),

    ADD CONSTRAINT transactions_must_either_onchain_or_offchain CHECK
        (
            (
                    (address IS NOT NULL OR txid IS NOT NULL OR vout IS NOT NULL OR confirmed_at IS NOT NULL OR
                     confirmed_at_block IS NOT NULL) -- if any of these fields are defined
                    AND
                    (memo IS NULL AND payment_request IS NULL AND preimage IS NULL AND
                     hashed_preimage IS NULL AND invoice_settled_at IS NULL AND
                     invoice_status IS NULL)) -- all of these have to be null
            OR
            (
                    (memo IS NOT NULL OR payment_request IS NOT NULL OR preimage IS NOT NULL OR
                     hashed_preimage IS NOT NULL OR
                     invoice_settled_at IS NULL AND invoice_status IS NULL) -- if any of these fields are defined
                    AND
                    (address IS NULL AND txid IS NULL AND vout IS NULL AND confirmed_at IS NULL AND
                     confirmed_at_block IS NULL) -- all of these have to be null
                )
        ),
    ADD CONSTRAINT transactions_txid_or_vout_cant_exist_alone CHECK (
            (vout IS NOT NULL AND txid IS NOT NULL)
            OR
            (vout IS NULL AND txid IS NULL) );

