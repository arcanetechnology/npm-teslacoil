#! /usr/bin/env bash

exe() { echo "\$ $@" ; "$@" ; }

set -e

exe sudo systemctl stop tlc.service
exe sudo systemctl start tlc.service
