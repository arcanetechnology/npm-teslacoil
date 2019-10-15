ALTER TABLE offchaintx
ADD COLUMN customer_order_id VARCHAR(256);

ALTER TABLE offchaintx
ADD CONSTRAINT unique_order_id_and_user_id UNIQUE (customer_order_id, user_id);