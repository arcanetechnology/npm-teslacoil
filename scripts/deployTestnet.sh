#! /usr/bin/env bash

exe() { echo "\$ $@" ; "$@" ; }

exe tlc db up
exe docker-compose up -d bitcoind
BITCOIND_IP=`docker inspect --format '{{json .NetworkSettings}}' bitcoind | jq .Networks.teslacoil_default.IPAddress --raw-output`
PREV_IP=`systemctl | grep -m1 teslacoil@ | grep -o -P '(?<=teslacoil).*(?=.service)'`
exe sudo systemctl stop tlc${PREV_IP}.service
exe sudo systemctl start tlc@"${BITCOIND_IP}".service
