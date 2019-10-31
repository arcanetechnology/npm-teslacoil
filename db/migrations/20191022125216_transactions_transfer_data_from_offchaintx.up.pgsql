-- transfer all data from offchaintx to transcations
INSERT INTO transactions
(user_id,
 payment_request,
 amount_milli_sat,
 settled_at,
 invoice_status,
 description,
 direction,
 preimage,
 hashed_preimage,
 callback_url,
 created_at,
 updated_at,
 deleted_at,
 memo,
 expiry,
 customer_order_id)
SELECT user_id,
       payment_request,
       amount_msat,
       settled_at,
       status,
       description,
       direction,
       decode(preimage, 'hex'),
       decode(hashed_preimage, 'hex'),
       callback_url,
       created_at,
       updated_at,
       deleted_at,
       memo,
       expiry,
       customer_order_id
FROM offchaintx;

-- delete the old table
DROP TABLE offchaintx;

-- update all onchain TXs to use millisatoshis instead of satoshis
UPDATE transactions SET amount_milli_sat = amount_sat * 1000 WHERE address != '';

-- delete the old satoshis field
ALTER TABLE transactions DROP COLUMN amount_sat;
