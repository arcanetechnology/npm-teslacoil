[![Coverage Report](https://gitlab.com/arcanecrypto/teslacoil/badges/master/coverage.svg)](https://gitlab.com/arcanecrypto/teslacoil/commits/master)

# Lightning Payment Processor

A custodial payment processor running in the lightning network using lnd.

## Getting started with development

1. Install direnv `sudo apt install direnv` and add `direnv hook fish | source`
   to your Fish config file (`$HOME/.config/fish/config.fish`)
   See here if you are using a different shell https://direnv.net/docs/hook.md

2. Create `.envrc` and fill in details (see defaults in `.envrc-example`)
3. Start the API: `make serve`

## Start the API

#### Regtest

```bash
make serve
```

#### Testnet

```bash
make serve-testnet
```

## DB management etc.

Run `tlc db` to see options.

## Testing

#### Basic tests

```bash
make test
```

#### Integration tests

These tests take a bit longer to run, as they create and destroy `lnd` and 
`bitcoind` nodes. 

```bash
make test-it
```

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
ln -sf $PWD/contrib/lncli.fish $HOME/.config/fish/completions/lncli.fish
make build && ./tlc fish-completion | source
```

## Docker

We use Docker and Docker Compose to manage infrastructure such as `lnd`, `bitcoind` and 
Postgres. 

### Spinning up cluster

Generally speaking, the Docker cluster should be managed through `make` targets. For example,
when doing `make serve`, containers are started for you with the correct configuration values.

If you want to spin up individual services by yourself, that can be done. For example, to 
up a cluster with `bitcoind`, two `lnd` nodes and a Postgres DB:


```bash
$ docker-compose up --detach alice bob bitcoind db# can also use -d
```

### Logs

Viewing logs from instances:

```bash
docker-compose logs # all logs
docker-compose logs alice # just alice
docker-compose logs -f bob #  trail bobs logs
```

It's also possible to view Postgres logs of executed SQL statements. These reside
in `/var/log/postgresql/postgres.log`. Some examples on how to view them: 

```bash
# simple log trailing
docker exec -it postgres tail -f /var/log/postgresql/postgres.log

# dump it to file, so you can inspect it with your favorite tool
# this streams data to pgdump.log in your current directory until 
# you do <CTRL+C>
docker exec -it postgres tail -f /var/log/postgresql/postgres.log > pgdump.log
```

### Winding cluster down

Winding cluster down:

```bash
docker-compose down
```

If you want to reset the state of your cluster, do:

```bash
rm -rf docker/.{alice,bob}/*
```

Be careful to not delete the `.alice` and `.bob` directories themselves, though. That's
going to screw up permissions once the containers get started.

### Deleting specific volumes

### Nuking Postgres

If you want to nuke Postgres and have a fresh DB:

```bash
make nuke_postgres
```

This rebuilds to image (picking up any changes you've made in the `Dockerfile`),
kills the container, wipes the data store and starts it again. 

#### VSCode

In Docker tab (Docker extension), "volumes" section at the bottom.
Right click, and then remove. Note that container must be removed
first.

#### Terminal

Easiest way I've found (assuming you want to delete Postgres data):

```bash
docker-compose rm db #
docker volume rm teslacoil_postgres # name is teslacoil_ + service name
```

#### Glossary
##### Payment
A payment is a lightning transaction sent by us, ie. a lightning transaction
 initiated by us using lncli.SendPayment()
##### Invoice
An invoice is a lightning transaction we receive, ie. all lightning
 transactions created using lncli.AddInvoice()
