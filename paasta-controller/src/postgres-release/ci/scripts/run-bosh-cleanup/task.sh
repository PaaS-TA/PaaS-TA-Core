#!/bin/bash -exu

function main() {
  bosh -t $BOSH_DIRECTOR cleanup --all
}

main
