-- We remove the balance column from users to reduce our attack vector
-- https://gitlab.com/arcanecrypto/teslacoil/merge_requests/97#note_233717997
ALTER TABLE users
    DROP COLUMN balance;