#! /usr/bin/env bash

# exit on error
set -e

# echo executed commands
set -o xtrace

BOB_IP=`docker inspect --format '{{json .NetworkSettings}}' bob | jq .Networks.teslacoil_default.IPAddress --raw-output`
BOB_PUBKEY=`./bob getinfo | jq --raw-output .identity_pubkey`
ALICE_IP=`docker inspect --format '{{json .NetworkSettings}}' alice | jq .Networks.teslacoil_default.IPAddress --raw-output`

ALICE_ADDR=`./alice newaddress p2wkh | jq --raw-output .address`
BOB_ADDR=`./bob newaddress p2wkh | jq --raw-output .address`

# give Alice some balance
./bitcoin-cli generatetoaddress 3 $ALICE_ADDR

sleep 5

# open channel from alice to bob with money on both sides
./alice openchannel --node_key $BOB_PUBKEY --connect $BOB_IP --local_amt 16000000 --push_amt 10000000

# confirm channel
./bitcoin-cli generatetoaddress 6 $ALICE_ADDR
