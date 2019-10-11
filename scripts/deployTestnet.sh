#! /usr/bin/env bash

exe() { echo "\$ $@" ; "$@" ; }

exe lpp db up
exe docker-compose up -d bitcoind
BITCOIND_IP=`docker inspect --format '{{json .NetworkSettings}}' bitcoind | jq .Networks.teslacoil_default.IPAddress --raw-output`
PREV_IP=`systemctl | grep -m1 teslacoil@ | grep -o -P '(?<=teslacoil).*(?=.service)'`
exe echo $PREV_IP
exe echo $BITCOIND_IP
exe sudo systemctl stop teslacoil@${PREV_IP}.service
exe sudo systemctl start teslacoil@"${BITCOIND_IP}".service
