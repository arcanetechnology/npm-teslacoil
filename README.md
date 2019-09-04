# Lightning Payment Processor

A custodial payment processor running in the lightning network using lnd.

## Installation

1. Install direnv `sudo apt install direnv` and add `direnv hook fish | source` to your ./config.fish

See here if you are using a different shell https://direnv.net/docs/hook.md

2. Create .envrc and fill inn details (see defaults in .envrc-example)
3. Install postgres and create a user and a sample DB by running these commands:
   ```
   sudo apt update && sudo apt install postgresql postgresql-contrib
   sudo -u postgres psql
   create user lpp with encrypted password 'password';
   create database lpp with owner lpp;
   grant all privileges on database lpp to lpp;
   ```
4. Create a password for the postgres user: This step is necessary to interact with the DB through bash commands. We need to be able to do this because we want to be able to run tests in parallell, which is easiest by creating one database per test package

   ```
   sudo -u postgres psql template1
   alter user postgres with encrypted password 'password';
   \q
   sudo systemctl restart postgresql.service
   ```

5. Add the password to your .envrc file, like `export PGPASSWORD="password"`
6. Migrate the db: `lpp db up`

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
