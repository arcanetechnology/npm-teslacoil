-- add all new constraints
ALTER TABLE transactions
    -- remove all constraints
    DROP CONSTRAINT txid_length,
    DROP CONSTRAINT transactions_txid_vout_key,
    DROP CONSTRAINT txid_or_vout_cant_exist_alone,
    DROP CONSTRAINT transactions_amount_sat_check,

    -- common constraints
    ADD CONSTRAINT transactions_must_either_be_onchain_or_offchain CHECK
        (
            (
                    (address IS NOT NULL) -- if address is defined
                    AND
                    (payment_request IS NULL AND memo IS NULL AND hashed_preimage IS NULL AND
                     invoice_status IS NULL) -- all offchain fields have to be null
                )
            OR
            (
                    (payment_request IS NOT NULL) -- if payment request is defined
                    AND
                    (address IS NULL AND txid IS NULL AND confirmed_at IS NULL AND
                     confirmed_at_block IS NULL) -- all onchain fields have to be null
                )
        ),
    ADD CONSTRAINT transactions_positive_amount_milli_sat CHECK (amount_milli_sat >= 0),

    -- onchain constraints
    ADD CONSTRAINT transactions_positive_vout CHECK (vout >= 0),
    ADD CONSTRAINT transactions_positive_expiry CHECK (expiry >= 0),
    ADD CONSTRAINT transactions_txid_and_vout_must_be_unique UNIQUE (txid, vout),
    ADD CONSTRAINT transactions_txid_length CHECK ( length(txid) = 64 ),
    ADD CONSTRAINT transactions_must_have_txid_if_confirmed_at_block CHECK
        (
            (confirmed_at_block IS NOT NULL AND txid IS NOT NULL)
            OR
            (txid IS NOT NULL AND confirmed_at_block IS NULL)
            OR
            (txid IS NULL AND confirmed_at_block IS NULL)
        ),
    ADD CONSTRAINT transactions_must_have_txid_if_confirmed_at CHECK
        (
            (confirmed_at IS NOT NULL AND txid IS NOT NULL)
            OR
            (txid IS NOT NULL AND confirmed_at IS NULL)
            OR
            (txid IS NULL AND confirmed_at IS NULL)
        ),
    ADD CONSTRAINT transactions_must_have_txid_or_payment_request_if_settled_at CHECK (
            settled_at IS NOT NULL AND (txid IS NOT NULL OR payment_request IS NOT NULL)
            OR
            settled_at IS NULL
        ),
    ADD CONSTRAINT transactions_txid_or_vout_cant_exist_alone CHECK
        (
            (vout IS NOT NULL AND txid IS NOT NULL)
            OR
            (vout IS NULL AND txid IS NULL)
        ),

    -- offchain constraints
    ADD CONSTRAINT transactions_hash_must_exist_if_preimage_is_defined CHECK
        (
            preimage IS NULL -- either preimage has to be null
            OR
            (preimage IS NOT NULL AND hashed_preimage IS NOT NULL) -- or preimage AND hashed_preimage must be defined
        );

