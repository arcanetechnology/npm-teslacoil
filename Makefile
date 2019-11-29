all: test build

PKG := gitlab.com/arcanecrypto/teslacoil
COMMIT := $(shell git describe --abbrev=40 --dirty)

# set `var build.Commit` to current commit (or version if on a tag)
LDFLAGS := -ldflags "-X $(PKG)/build.commit=$(COMMIT)"

BINARIES := tlc tlc-dev
COVERAGE_ARTIFACTS := coverage.out coverage.html

.PHONY: build
build:
	go build ${LDFLAGS} -o tlc ./cmd/tlc/main.go

images:
	docker-compose build --parallel

deploy-testnet: install
	./scripts/deployTestnet.sh

deploy-mainnet: install backup_db
	./scripts/deployMainnet.sh

backup_db:
	pg_dump teslacoil > /home/admin/teslacoil-backups/$(shell date --iso=date).backup

start-db:
	if [ -z `docker-compose ps -q db` ]; then docker-compose up -d db && sleep 3; fi
	if [ `uname` = "Darwin" ] && [ -z `netstat | grep 5432` ]; then pg_ctl -D /usr/local/var/postgres start; fi

start-regtest-alice: 
	 ZMQPUBRAWTX_PORT=23473 ZMQPUBRAWBLOCK_PORT=23472 BITCOIN_NETWORK=regtest docker-compose up -d alice 

drop: build start-db
	./tlc db drop --force

clean:
	rm -f ${BINARIES} ${COVERAGE_ARTIFACTS}

install:
	go install ${LDFLAGS} ./...

# If the first argument is "test-only"...
ifeq (test-only,$(firstword $(MAKECMDGOALS)))
  # use the rest as arguments for "test-only"
  TEST_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  # ...and turn them into do-nothing targets
  $(eval $(RUN_ARGS):;@:)
endif


serve: images
	env BITCOIN_NETWORK=regtest docker-compose up dev 

serve-testnet: images
	env BITCOIN_NETWORK=testnet docker-compose up dev 

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

coverage-report: 
	go test ./... -coverprofile coverage.out
	go tool cover -html=coverage.out -o coverage.html

nuke_postgres:
	docker-compose build --no-cache db 
	docker-compose rm --force --stop -v db
	docker volume rm teslacoil_postgres
	docker-compose up --detach
