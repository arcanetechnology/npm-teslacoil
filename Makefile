PAYMENTS_TEST_DB=$(DATABASE_TEST_NAME)_payments
USERS_TEST_DB=$(DATABASE_TEST_NAME)_users

all: test build
build:
	go build ./...
install:
	-psql -U postgres -w -c "DROP DATABASE $(USERS_TEST_DB);" -h $(DATABASE_TEST_HOST)
	-psql -U postgres -w -c "DROP DATABASE $(PAYMENTS_TEST_DB);" -h $(DATABASE_TEST_HOST)

	-psql -U postgres -w -c "CREATE DATABASE $(USERS_TEST_DB) with owner $(DATABASE_TEST_USER);" -h $(DATABASE_TEST_HOST)
	-psql -U postgres -w -c "GRANT ALL PRIVILEGES ON DATABASE $(USERS_TEST_DB) TO $(DATABASE_TEST_USER);" -h $(DATABASE_TEST_HOST)

	-psql -U postgres -w -c "CREATE DATABASE $(PAYMENTS_TEST_DB) with owner $(DATABASE_TEST_USER);" -h $(DATABASE_TEST_HOST)
	-psql -U postgres -w -c "GRANT ALL PRIVILEGES ON DATABASE $(PAYMENTS_TEST_DB) TO $(DATABASE_TEST_USER);" -h $(DATABASE_TEST_HOST)

	$(MAKE) build
test:
	go test -v ./...