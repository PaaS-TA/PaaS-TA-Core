#!/bin/bash -exu

preflight_check() {
  set +x
  test -n "${BOSH_DIRECTOR}"
  test -n "${BOSH_CLIENT}"
  test -n "${BOSH_CLIENT_SECRET}"
  test -n "${BOSH_CA_CERT}"
  test -n "${DEPLOYMENT_NAME}"
  set -x
}

function main() {
  preflight_check
  export BOSH_ENVIRONMENT="https://${BOSH_DIRECTOR}:25555"
  bosh -n -d $DEPLOYMENT_NAME run-errand acceptance_tests
}

main
