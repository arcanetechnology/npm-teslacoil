#! /usr/bin/env bash

# exit on error
set -e

#function to display and execute commands
exe() { echo "\$ $@" ; "$@" ; }

ZMQ_INFO=`bitcoin-cli -testnet getzmqnotifications`
ZMQPUBRAWTX_PORT=`echo $ZMQ_INFO | jq --raw-output ' .[] | select(.type == "pubrawtx") | .address' | awk -F: '{print $NF}'`
ZMQPUBRAWBLOCK_PORT=`echo $ZMQ_INFO | jq --raw-output ' .[] | select(.type == "pubrawblock") | .address' | awk -F: '{print $NF}'`

exe docker-compose build --parallel > /dev/null
exe env BITCOIN_NETWORK=testnet \
    docker-compose up -d 

if [[ -f $HOME/.bitcoin/testnet3/.cookie ]]; then
    COOKIE=`cat $HOME/.bitcoin/testnet3/.cookie`
    RPCUSER=`echo $COOKIE | cut --delimiter : --fields 1`
    RPCPASS=`echo $COOKIE | cut --delimiter : --fields 2`
fi

exe ./lpp-dev \
    --bitcoind.rpcuser $RPCUSER \
    --bitcoind.rpcpassword $RPCPASS \
    --loglevel debug \
	--network testnet \
	--zmqpubrawblock localhost:${ZMQPUBRAWBLOCK_PORT} \
	--zmqpubrawtx localhost:${ZMQPUBRAWTX_PORT} \
	serve
