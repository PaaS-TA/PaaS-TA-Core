#!/bin/bash -eu

preflight_check() {
  set +x
  test -n "${BOSH_DIRECTOR}"
  test -n "${BOSH_PUBLIC_IP}"
  test -n "${BOSH_CLIENT}"
  test -n "${BOSH_CLIENT_SECRET}"
  test -n "${BOSH_CA_CERT}"
  test -n "${OLD_CF_RELEASE}"
  test -n "${CF_DEPLOYMENT}"
  test -n "${API_USER}"
  test -n "${API_PASSWORD}"
  test -n "${STEMCELL_VERSION}"
  set -x
}

function upload_remote_release() {
  local release_url=$1
  wget --quiet "${release_url}" -O remote_release.tgz
  bosh upload-release remote_release.tgz
}

generate_stub() {
  local vm_prefix
  local cf1_domain
  local apps_domain
  vm_prefix="${CF_DEPLOYMENT}-"
  apps_domain="apps.${CF_DEPLOYMENT}.microbosh"
  cf1_domain="cf1.${CF_DEPLOYMENT}.microbosh"
  cat <<EOF
---
common_data:
  <<: (( merge ))
  VmNamePrefix: ${vm_prefix}
  cf1_domain: ${cf1_domain}
  env_name: ${CF_DEPLOYMENT}
  apps_domain: ${apps_domain}
  api_user: ${API_USER}
  api_password: ${API_PASSWORD}
  Bosh_ip: ${BOSH_DIRECTOR}
  Bosh_public_ip: ${BOSH_PUBLIC_IP}
  stemcell_version: ${STEMCELL_VERSION}
  default_env:
    bosh:
      password: ~
      keep_root_password: true
  cf_version: ${OLD_CF_RELEASE}
EOF
}

function main(){
  local root="${1}"
  preflight_check
  export BOSH_ENVIRONMENT="https://${BOSH_DIRECTOR}:25555"

  mkdir ${root}/stubs

  pushd ${root}/stubs
    generate_stub > data.yml
  popd

  spiff merge \
    "${root}/postgres-ci-env/deployments/cf/pgci-cf${OLD_CF_RELEASE}.yml" \
    "${root}/postgres-ci-env/deployments/common/properties.yml" \
    "${root}/stubs/data.yml" \
    "${root}/postgres-ci-env/deployments/common/common.yml" > "${root}/${CF_DEPLOYMENT}.yml"

  upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/cf-release?v=${OLD_CF_RELEASE}"

  bosh -n deploy -d "${CF_DEPLOYMENT}" "${root}/${CF_DEPLOYMENT}.yml"
}


main "${PWD}"
