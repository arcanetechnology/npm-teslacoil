# Lightning Payment Processor

A custodial payment processor running in the lightning network using lnd.

## Installation

1. Create .envrc and will inn details (see defaults in .envrc-example)
2. Run the `setuplpp` script in the scripts/ folder
   Install postgres and create a new database `sudo mysql -e "create database lpp;"`
3.

Run: `go get` to install dependencies

## Start the API

`lpp serve`

## DB management etc.

Run `lpp db` to see options.
