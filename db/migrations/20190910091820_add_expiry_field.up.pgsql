ALTER TABLE offchaintx ADD COLUMN expiry bigint;
COMMENT ON COLUMN offchaintx.expiry IS 'lightning invoice expiry in seconds';
