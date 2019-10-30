all: test build

# If we're on a tag, binary name is tlc, else tlc-dev
TLC := $(shell git describe --exact-match HEAD 2>/dev/null && echo tlc || echo tlc-dev)
BINARIES := tlc tlc-dev

.PHONY: build
build:
	go build -o ${TLC} ./cmd/tlc/main.go

deploy-testnet: install
	./scripts/deployTestnet.sh

deploy-mainnet: install
	./scripts/deployMainnet.sh

start-db:
	if [ -z `docker-compose ps -q db` ]; then docker-compose up -d db && sleep 3; fi
	if [ `uname` = "Darwin" ] && [ -z `netstat | grep 5432` ]; then pg_ctl -D /usr/local/var/postgres start; fi

start-regtest-alice: 
	 ZMQPUBRAWTX_PORT=23473 ZMQPUBRAWBLOCK_PORT=23472 BITCOIN_NETWORK=regtest docker-compose up -d alice 

migrate-up: build start-db
	./tlc-dev db up

drop: build start-db
	./tlc-dev db drop --force

dummy-data: build start-db migrate-up start-regtest-alice
	./tlc-dev --network regtest db dummy --force --only-once
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

serve: dummy-data build
	env BITCOIN_NETWORK=regtest ./scripts/serve.sh

serve-testnet: dummy-data build
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

test-verbose:
	go test ./... -v

nuke_postgres:
	docker-compose build --no-cache db 
	docker-compose rm --force --stop -v db
	docker volume rm teslacoil_postgres
	docker-compose up --detach
