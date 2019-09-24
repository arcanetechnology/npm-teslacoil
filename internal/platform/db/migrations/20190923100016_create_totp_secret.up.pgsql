ALTER TABLE users
ADD COLUMN totp_secret VARCHAR(256);

ALTER TABLE users
ADD COLUMN confirmed_totp_secret BOOLEAN;
