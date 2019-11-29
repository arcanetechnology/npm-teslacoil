#!/usr/bin/env bash

# exit from script if error was raised.
set -e

# error function is used within a bash function in order to send the error
# message directly to the stderr output and exit.
error() {
    echo "$1" > /dev/stderr
    exit 0
}

# return is used within bash function in order to return the value.
return() {
    echo "$1"
}

# set_default function gives the ability to move the setting of default
# env variable from docker file to the script thereby giving the ability to the
# user override it durin container start.
set_default() {
    # docker initialized env variables with blank string and we can't just
    # use -z flag as usually.
    BLANK_STRING='""'

    VARIABLE="$1"
    DEFAULT="$2"

    if [[ -z "$VARIABLE" || "$VARIABLE" == "$BLANK_STRING" ]]; then

        if [ -z "$DEFAULT" ]; then
            error "You should specify default variable"
        else
            VARIABLE="$DEFAULT"
        fi
    fi

   return "$VARIABLE"
}

# Set default variables if needed.
RPCUSER=$(set_default "$RPCUSER" "devuser")
RPCPASS=$(set_default "$RPCPASS" "devpass")
RPCHOST=$(set_default "$RPCHOST" "localhost")
DEBUG=$(set_default "$DEBUG" "debug")
BITCOIN_NETWORK=$(set_default "$BITCOIN_NETWORK" "regtest")

PARAMS="\
--noseedbackup \
--logdir=/data \
--bitcoin.active \
--bitcoin.$BITCOIN_NETWORK \
--bitcoin.node=bitcoind \
--rpclisten=0.0.0.0:10009 \
--bitcoind.rpchost=$RPCHOST \
--bitcoind.rpcuser=$RPCUSER \
--bitcoind.rpcpass=$RPCPASS \
--bitcoind.zmqpubrawtx=tcp://$RPCHOST:$ZMQPUBRAWTX_PORT
--bitcoind.zmqpubrawblock=tcp://$RPCHOST:$ZMQPUBRAWBLOCK_PORT
--debuglevel=$DEBUG
--tlsextradomain=bob
--tlsextradomain=alice" 
# last two lines are necessary for inter-container communication with TLS


echo "Command: lnd $PARAMS $@"
exec lnd $PARAMS "$@"
