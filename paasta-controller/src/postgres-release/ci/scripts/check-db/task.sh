#!/bin/bash -exu

function main() {

  local root="${1}"
  local api_endpoint="api.apps.${CF_DEPLOYMENT}.microbosh"

  cf api ${api_endpoint} --skip-ssl-validation
  set +x
  cf auth $API_USER $API_PASSWORD
  set -x
  cf target -o ${CF_DEPLOYMENT} -s ${CF_DEPLOYMENT}

  cf apps
  curl --fail dora.apps.${CF_DEPLOYMENT}.microbosh

}

main "${PWD}"
