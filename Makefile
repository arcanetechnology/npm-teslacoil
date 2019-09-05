all: test build

build:
	go build ./...
install:
	go install ./...

# If the first argument is "test-only"...
ifeq (test-only,$(firstword $(MAKECMDGOALS)))
  # use the rest as arguments for "test-only"
  TEST_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  # ...and turn them into do-nothing targets
  $(eval $(RUN_ARGS):;@:)
endif

test-only: 
	go test ./... -run ${TEST_ARGS}

test:
	go test ./...

test_verbose:
	go test ./... -v
