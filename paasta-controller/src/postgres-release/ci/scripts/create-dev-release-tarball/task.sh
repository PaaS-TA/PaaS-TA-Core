#!/bin/bash -eu

preflight_check() {
  set +x
  test -n "${BOSH_DIRECTOR}"
  test -n "${BOSH_CLIENT}"
  test -n "${BOSH_CLIENT_SECRET}"
  test -n "${BOSH_CA_CERT}"
  test -n "${REL_NAME}"
  test -n "${REL_VERSION}"
  set -x
}

function main(){
  local root="${1}"
  preflight_check
  export BOSH_ENVIRONMENT="https://${BOSH_DIRECTOR}:25555"
  echo "${BOSH_CA_CERT}" > ${root}/ca_cert
  /opt/rubies/ruby-2.2.4/bin/bosh --ca-cert=${root}/ca_cert -u ${BOSH_CLIENT} -p ${BOSH_CLIENT_SECRET} target ${BOSH_ENVIRONMENT}

  pushd ${root}/dev-release
  /opt/rubies/ruby-2.2.4/bin/bosh -t ${BOSH_DIRECTOR} create release --force --with-tarball --version "${REL_VERSION}" --name "${REL_NAME}"
  cp dev_releases/${REL_NAME}/${REL_NAME}-${REL_VERSION}.tgz ${root}/dev-release-tarball
  popd
}

main "${PWD}"
