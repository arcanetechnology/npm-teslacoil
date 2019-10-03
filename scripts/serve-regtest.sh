#! /usr/bin/env bash

# exit on error
set -e

#function to display and execute commands
exe() { echo "\$ $@" ; "$@" ; }

exe docker-compose build --parallel > /dev/null
exe env RPCHOST=blockchain \
    BITCOIN_NETWORK=regtest \
    docker-compose up -d db bob alice 

BITCOIND_IP=`docker inspect --format '{{json .NetworkSettings}}' bitcoind | jq .Networks.teslacoil_default.IPAddress --raw-output`
exe ./lpp-dev \
    --bitcoind.rpcuser $RPCUSER \
    --bitcoind.rpcpassword $RPCPASS \
    --bitcoind.rpchost $BITCOIND_IP \
	--zmqpubrawblock ${BITCOIND_IP}:${ZMQPUBRAWBLOCK_PORT} \
	--zmqpubrawtx ${BITCOIND_IP}:${ZMQPUBRAWTX_PORT} \
	--network regtest \
	serve
