ALTER TABLE offchaintx ADD COLUMN memo VARCHAR(256);
COMMENT ON COLUMN offchaintx.memo IS 'memo stored on lightning invoice';
COMMENT ON COLUMN offchaintx.description IS 'personal/internal description of payment';
