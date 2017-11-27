#!/bin/bash -exu

preflight_check() {
  set +x
  test -n "${BOSH_DIRECTOR}"
  test -n "${BOSH_CLIENT}"
  test -n "${BOSH_CLIENT_SECRET}"
  test -n "${BOSH_CA_CERT}"
  set -x
}

function main() {
  preflight_check
  export BOSH_ENVIRONMENT="https://${BOSH_DIRECTOR}:25555"
  bosh -n clean-up --all
}

main
