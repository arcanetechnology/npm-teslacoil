#! /usr/bin/env bash

exe() { echo "\$ $@" ; "$@" ; }

exe teslacoil db up
exe sudo systemctl stop teslacoil.service
exe sudo systemctl start teslacoil.service
