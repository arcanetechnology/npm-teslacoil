/* User and DB for running API locally */
CREATE USER teslacoil;
CREATE DATABASE teslacoil;
GRANT ALL PRIVILEGES ON DATABASE teslacoil TO teslacoil;

/* Test DBs and user */
CREATE USER teslacoil_test;
CREATE DATABASE teslacoil_payments;
GRANT ALL PRIVILEGES ON DATABASE teslacoil_payments to teslacoil_test;
CREATE DATABASE teslacoil_users;
GRANT ALL PRIVILEGES ON DATABASE teslacoil_users to teslacoil_test;