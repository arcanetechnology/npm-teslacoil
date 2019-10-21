#! /usr/bin/env bash

# exit on error
set -e

if [[ -z "$ZMQPUBRAWBLOCK_PORT" ]]; then
    export ZMQPUBRAWBLOCK_PORT=23472
fi

if [[ -z "$ZMQPUBRAWTX_PORT" ]]; then
    export ZMQPUBRAWTX_PORT=23473
fi

if [[ -z "$RPCUSER" ]]; then
    export RPCUSER=devuser
fi

if [[ -z "$RPCPASS" ]]; then
    export RPCPASS=devpass
fi


#function to display and execute commands
exe() { echo "\$ $@" ; "$@" ; }

exe docker-compose build --parallel
exe docker-compose up -d

BITCOIND_IP=`docker inspect --format '{{json .NetworkSettings}}' bitcoind | jq .Networks.teslacoil_default.IPAddress --raw-output`
exe ./tlc-dev \
	--network $BITCOIN_NETWORK \
	--logging.level debug \
	serve \
	--db.user tlc \
	--db.password password \
	--db.port 5434 \
	--db.migrateup \
	--dummy.gen-data \
	--dummy.force \
	--dummy.only-once \
    --bitcoind.rpcuser $RPCUSER \
    --bitcoind.rpcpassword $RPCPASS \
    --bitcoind.rpchost $BITCOIND_IP \
	--bitcoind.zmqpubrawblock $ZMQPUBRAWBLOCK_PORT \
	--bitcoind.zmqpubrawtx $ZMQPUBRAWTX_PORT \
    --lnd.dir docker/.alice \
    --rsa-jwt-key contrib/sample-private-pkcs1-rsa.pem
