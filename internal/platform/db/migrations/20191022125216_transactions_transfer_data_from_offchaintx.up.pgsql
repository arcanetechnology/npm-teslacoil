-- transfer all data from offchaintx to transcations
INSERT INTO transactions
(user_id,
 payment_request,
 amount_sat,
 invoice_settled_at,
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
       amount_sat,
       settled_at,
       status,
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
       customer_order_id
FROM offchaintx;

-- delete the old table
DROP TABLE offchaintx;
