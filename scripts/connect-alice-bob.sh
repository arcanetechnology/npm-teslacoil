#! /usr/bin/env bash

# exit on error
set -e

# echo executed commands
set -o xtrace

# if both alice and bob has money, exit script
ALICE_BALANCE=`./alice channelbalance | jq --raw-output .balance`
BOB_BALANCE=`./bob channelbalance | jq --raw-output .balance`

if [[ "$ALICE_BALANCE" -gt "0" && "$BOB_BALANCE" -gt 0 ]]; then
    echo Both Alice \($ALICE_BALANCE\) and Bob \($BOB_BALANCE\) has money, skipping rest of script
    exit 0
fi

BOB_IP=`docker inspect --format '{{json .NetworkSettings}}' bob | jq .Networks.teslacoil_default.IPAddress --raw-output`
BOB_PUBKEY=`./bob getinfo | jq --raw-output .identity_pubkey`
ALICE_IP=`docker inspect --format '{{json .NetworkSettings}}' alice | jq .Networks.teslacoil_default.IPAddress --raw-output`

ALICE_ADDR=`./alice newaddress p2wkh | jq --raw-output .address`

# give Alice some balance
./bitcoin-cli generatetoaddress 101 $ALICE_ADDR

sleep 2

# open channel from alice to bob with money on both sides
./alice openchannel --node_key $BOB_PUBKEY --connect $BOB_IP --local_amt 1000000 --push_amt 500000

# confirm channel
./bitcoin-cli generatetoaddress 6 $ALICE_ADDR