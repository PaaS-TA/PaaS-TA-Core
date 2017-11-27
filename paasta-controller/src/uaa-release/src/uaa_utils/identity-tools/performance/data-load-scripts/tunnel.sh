#!/usr/bin/env bash

if [ "$1" == "start" ]; then
 ssh -i $2 -L 23306:10.10.8.11:3306 vcap@54.174.40.160 -N &
 echo $! > tunnel.pid
 exit 0
elif [ "$1" == "stop" ]; then
 kill -9 `cat tunnel.pid`
 rm tunnel.pid
fi
