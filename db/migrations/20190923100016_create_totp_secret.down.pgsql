ALTER TABLE users
DROP COLUMN totp_secret;

ALTER TABLE users
DROP COLUMN confirmed_totp_secret;