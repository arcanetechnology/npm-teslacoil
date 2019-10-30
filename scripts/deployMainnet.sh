#! /usr/bin/env bash

exe() { echo "\$ $@" ; "$@" ; }

exe tlc db up
exe sudo systemctl stop tlc.service
exe sudo systemctl start tlc.service
