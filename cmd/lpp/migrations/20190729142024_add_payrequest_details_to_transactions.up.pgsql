ALTER TABLE transactions
ADD COLUMN pre_image VARCHAR(256),
ADD COLUMN hashed_pre_image VARCHAR(256),
ADD COLUMN callback_url TEXT;
