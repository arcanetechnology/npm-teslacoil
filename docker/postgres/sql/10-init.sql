/* User and DB for running API locally */
CREATE USER lpp;
CREATE DATABASE lpp;
GRANT ALL PRIVILEGES ON DATABASE lpp TO lpp;

/* Test DBs and user */
CREATE USER lpp_test;
CREATE DATABASE lpp_payments;
GRANT ALL PRIVILEGES ON DATABASE lpp_payments to lpp_test;
CREATE DATABASE lpp_users;
GRANT ALL PRIVILEGES ON DATABASE lpp_users to lpp_test;