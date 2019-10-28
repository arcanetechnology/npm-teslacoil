-- add all new constraints
ALTER TABLE transactions
    -- remove all constraints
    DROP CONSTRAINT txid_length,
    DROP CONSTRAINT transactions_txid_vout_key,
    DROP CONSTRAINT txid_or_vout_cant_exist_alone,
    DROP CONSTRAINT transactions_amount_sat_check,

    ADD CONSTRAINT transactions_positive_vout CHECK (vout >= 0),

    ADD CONSTRAINT transactions_positive_expiry CHECK (expiry >= 0),

    ADD CONSTRAINT transactions_txid_and_vout_must_be_unique UNIQUE (txid, vout),

    ADD CONSTRAINT transactions_txid_length CHECK ( length(txid) = 64 ),

    ADD CONSTRAINT transactions_onchain_must_have_address_if_has_txid CHECK
        (txid IS NULL OR (txid IS NOT NULL AND address IS NOT NULL)),

    ADD CONSTRAINT transactions_onchain_must_have_txid_if_confirmed_or_settled CHECK
        (confirmed_at IS NULL AND
         confirmed_at_block IS NULL
            OR -- either confirmed_at AND confirmed_at_block have to be null
         (
             ((confirmed_at_block IS NOT NULL OR
               confirmed_at IS NOT NULL) AND
              txid IS NOT NULL))), -- or either confirmed_at_block or confirmed_at AND txid have to be defined

    ADD CONSTRAINT transactions_txid_or_vout_cant_exist_alone CHECK
        (
            (vout IS NOT NULL AND txid IS NOT NULL)
            OR
            (vout IS NULL AND txid IS NULL)
        ),

    -- offchain/lightning constraints
    ADD CONSTRAINT transactions_invoice_status_must_exist_if_payment_request_exists CHECK
        (payment_request IS NULL OR ( -- either pay_req has to be null
                invoice_status IS NOT NULL AND
                payment_request IS NOT NULL -- or invoice_status AND payment_request have to be defined
            )),

    ADD CONSTRAINT transactions_hash_must_exist_if_preimage_is_defined CHECK
        (
            preimage IS NULL OR ( -- either preimage has to be null
                preimage IS NOT NULL AND
                hashed_preimage IS NOT NULL) -- or preimage AND hashed_preimage must be defined
        ),

    -- at this point, we know that preimage must be defined if hashed_preimage is defined
    -- we also know that a invoice_status must be defined if a payment_requeset is defined
    -- therefore we only check for memo, hashed_preimage, and invoice_settled_at
    ADD CONSTRAINT transactions_payment_request_must_exist_for_other_fields_to_exist CHECK
        (
            (payment_request IS NULL AND memo IS NULL AND
             hashed_preimage IS NULL) -- either all have to be null
            OR
            (payment_request IS NOT NULL) -- or payment_request must not be null
        ),

    -- common constraints
    ADD CONSTRAINT transactions_must_either_be_onchain_or_offchain CHECK
        (
            (
                    (address IS NOT NULL OR txid IS NOT NULL) -- if address or txid is defined
                    AND
                    (payment_request IS NULL)) -- payment request have to be null
            OR
            (
                    (payment_request IS NOT NULL) -- if payment request is defined
                    AND
                    (address IS NULL AND txid IS NULL) -- address and txid have to be undefined
                )
        ),
    ADD CONSTRAINT transactions_amount_milli_sat_must_be_greater_than_0 CHECK
        (amount_milli_sat >= 0);
