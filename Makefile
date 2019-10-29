all: test build-lpp

# If we're on a tag, binary name is lpp, else lpp-dev
LPP := $(shell git describe --exact-match HEAD 2>/dev/null && echo lpp || echo lpp-dev)
BINARIES := lpp lpp-dev

build-lpp:
	go build -o ${LPP} main.go

deploy-testnet: install
	./scripts/deployTestnet.sh

deploy-mainnet: install
	./scripts/deployMainnet.sh

start-db:
	if [ -z `docker-compose ps -q db` ]; then docker-compose up -d db && sleep 3; fi
	if [ `uname` = "Darwin" ] && [ -z `netstat | grep 5432` ]; then pg_ctl -D /usr/local/var/postgres start; fi

start-regtest-alice: 
	 ZMQPUBRAWTX_PORT=23473 ZMQPUBRAWBLOCK_PORT=23472 BITCOIN_NETWORK=regtest docker-compose up -d alice 

migrate-db-up: build-lpp start-db
	./lpp-dev db up

drop-db: build-lpp start-db
	./lpp-dev db drop --force

dummy-data: build-lpp start-db migrate-db-up start-regtest-alice
	./lpp-dev --network regtest db dummy --force --only-once
	docker-compose stop alice bitcoind


clean: 
	rm -f ${BINARIES}

install:
	go install ./...

# If the first argument is "test-only"...
ifeq (test-only,$(firstword $(MAKECMDGOALS)))
  # use the rest as arguments for "test-only"
  TEST_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  # ...and turn them into do-nothing targets
  $(eval $(RUN_ARGS):;@:)
endif

serve: dummy-data build-lpp 
	env BITCOIN_NETWORK=regtest ./scripts/serve.sh

serve-testnet: dummy-data build-lpp 
	env BITCOIN_NETWORK=testnet ./scripts/serve.sh

test-only: 
	go test ./... -run ${TEST_ARGS}

test:
	go test ./...
	golangci-lint run

test-it:
	go test ./... -tags integration

lint: 
	golangci-lint run	

test_verbose:
	go test ./... -v

nuke_postgres:
	docker-compose build --no-cache db 
	docker-compose rm --force --stop -v db
	docker volume rm teslacoil_postgres
	docker-compose up --detach
