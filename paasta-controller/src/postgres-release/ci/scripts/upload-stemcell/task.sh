#!/bin/bash -eu

preflight_check() {
  set +x
  test -n "${BOSH_DIRECTOR}"
  test -n "${BOSH_CLIENT}"
  test -n "${BOSH_CLIENT_SECRET}"
  test -n "${BOSH_CA_CERT}"
  test -n "${STEMCELL_VERSION}"
  set -x
}

function upload_stemcell() {
  if [ "${STEMCELL_VERSION}" == "latest" ]; then
    wget --quiet "https://bosh.io/d/stemcells/bosh-softlayer-xen-ubuntu-trusty-go_agent" --output-document=stemcell.tgz
  else
    wget --quiet "https://bosh.io/d/stemcells/bosh-softlayer-xen-ubuntu-trusty-go_agent?v=${STEMCELL_VERSION}" --output-document=stemcell.tgz
  fi
  bosh upload-stemcell stemcell.tgz
}

function main(){
  local root="${1}"
  preflight_check
  export BOSH_ENVIRONMENT="https://${BOSH_DIRECTOR}:25555"
  upload_stemcell
}

main "${PWD}"
