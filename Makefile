all: test build-lpp

# If we're on a tag, binary name is lpp, else lpp-dev
LPP := $(shell git describe --exact-match HEAD 2>/dev/null && echo lpp || echo lpp-dev)
BINARIES := lpp lpp-dev

build-lpp:
	go build -o ${LPP} ./cmd/lpp

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

serve: build-lpp
	env BITCOIN_NETWORK=regtest ./scripts/serve.sh

serve-testnet: build-lpp
	env BITCOIN_NETWORK=testnet ./scripts/serve.sh

serve-testnet: build-lpp
	docker-compose down
	docker-compose up -d db
	systemctl start lnd
	./lpp-dev serve

test-only: 
	go test ./... -run ${TEST_ARGS}

test:
	go test ./...

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
