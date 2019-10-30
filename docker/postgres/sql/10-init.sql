/* User and DB for running API locally */
CREATE USER tlc;
CREATE DATABASE tlc;
GRANT ALL PRIVILEGES ON DATABASE tlc TO tlc;

/* Test DBs and user */
CREATE USER tlc_test;
CREATE DATABASE tlc_payments;
GRANT ALL PRIVILEGES ON DATABASE tlc_payments to tlc_test;
CREATE DATABASE tlc_users;
GRANT ALL PRIVILEGES ON DATABASE tlc_users to tlc_test;
