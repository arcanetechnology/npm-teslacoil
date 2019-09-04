# Lightning Payment Processor

A custodial payment processor running in the lightning network using lnd.

## Installation

1. Install direnv `sudo apt install direnv` and add `direnv hook fish | source`
   to your Fish config file (`$HOME/.config/fish/config.fish`)
   See here if you are using a different shell https://direnv.net/docs/hook.md

2. Create `.envrc` and fill in details (see defaults in `.envrc-example`)
3. Build and install `lpp`: `go install ./...`
4. Start the LND/`btcd`/Postgres cluster: `docker-compose up -d`
5. Migrate the db: `lpp db up`

Run: `go get` to install dependencies

## Start the API

`lpp serve`

## DB management etc.

Run `lpp db` to see options.

## LND simnet for development

Fill inn instructions here.

## Testing

To run basic tests use `make test`.

To run tests using lnd on simnet use `make test tags="lnd"`.
This does however require you to have one or two simnet lnd nodes running.
See instructions above.

## General coding guidelines

There is a max length of 80 characters. The only exception is
strings

#### Errors

Because error messages are frequently chained together, message strings should NOT be capitalized, should NOT end with a ., and should avoid newlines. When designing error messages, be deliberate, so that each one is a meaningful description of the problem with sufficient and relevant detail. Also, be consistent, so that errors returned by the same function/package are similar in form and can be dealt with in the same way.
In general, the call f(x) is responsible for reporting the attempted operation f and the argument value x as they relate to the context of the error. The caller is responsible for adding further information that it has but the call f(x) does not.

Lets move on to the second strategy for handling errors. For errors that represent transient or unpredictable problems, it may make sense to retry the failed operation, possibly with a delay between tries, and perhaps with a limit on the number of attempts or the time spentr trying before giving up entirely.

Third, if progress is impossible, the caller can print the error and stop the program gracefully, but this course of action should generally be reserved for the main package of a program.

So, all error logging occurs in the top level package.

## Shell autocomplete

### Fish

```shell
$ ln -sf $PWD/contrib/lpp.fish $HOME/.config/fish/completions/lpp.fish
$ ln -sf $PWD/contrib/lncli.fish $HOME/.config/fish/completions/lncli.fish
```

## Docker

### Spinning up cluster

Spinning up local cluster with `btcd`, two LND nodes and Postgres DB:

```bash
$ docker-compose up --detach # can also use -d
```

### Logs

Viewing logs from instances:

```bash
$ docker-compose logs # all logs
$ docker-compose logs alice # just alice
$ docker-compose logs -f bob #  trail bobs logs
```

### Winding cluster down

Winding cluster down:

```bash
$ docker-compose down
```

If you want to reset the state of your cluster, do:

```bash
$ rm -rf docker/.{alice,bob}/*
```

Be careful to not delete the `.alice` and `.bob` directories themselves, though. That's
going to screw up permissions once the containers get started.

### Deleting specific volumes

#### VSCode

In Docker tab (Docker extension), "volumes" section at the bottom.
Right click, and then remove. Note that container must be removed
first.

#### Terminal

Easiest way I've found (assuming you want to delete Postgres data):

```bash
$ docker-compose rm db #
$ docker volume rm teslacoil_postgres # name is teslacoil_ + service name
```
