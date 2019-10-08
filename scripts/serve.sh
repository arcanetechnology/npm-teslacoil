#! /usr/bin/env bash

# exit on error
set -e

if [[ -z "$ZMQPUBRAWBLOCK_PORT" ]]; then
    export ZMQPUBRAWBLOCK_PORT=23472
fi

if [[ -z "$ZMQPUBRAWTX_PORT" ]]; then
    export ZMQPUBRAWTX_PORT=23473
fi

#function to display and execute commands
exe() { echo "\$ $@" ; "$@" ; }

exe docker-compose build --parallel
exe docker-compose up -d

BITCOIND_IP=`docker inspect --format '{{json .NetworkSettings}}' bitcoind | jq .Networks.teslacoil_default.IPAddress --raw-output`
exe ./lpp-dev \
    --bitcoind.rpcuser $RPCUSER \
    --bitcoind.rpcpassword $RPCPASS \
    --bitcoind.rpchost $BITCOIND_IP \
	--network $BITCOIN_NETWORK \
	--zmqpubrawblock ${BITCOIND_IP}:${ZMQPUBRAWBLOCK_PORT} \
	--zmqpubrawtx ${BITCOIND_IP}:${ZMQPUBRAWTX_PORT} \
	serve
