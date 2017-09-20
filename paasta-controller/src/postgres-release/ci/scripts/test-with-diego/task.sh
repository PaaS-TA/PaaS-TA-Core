#!/bin/bash -exu

function main() {
  bosh -t $BOSH_DIRECTOR download manifest $DEPLOYMENT_NAME manifest.yml

  bosh -n --color -t $BOSH_DIRECTOR -d manifest.yml run errand acceptance_tests
}

main
