DROP TABLE transactions;
DROP TYPE IF EXISTS direction; -- Must Drop type after dropping table that depends on it.
DROP TYPE IF EXISTS status;