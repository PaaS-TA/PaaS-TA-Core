#!/bin/bash -eu

preflight_check() {
  set +x
  test -n "${BOSH_DIRECTOR}"
  test -n "${BOSH_USER}"
  test -n "${BOSH_PASSWORD}"
  set -x
}

deploy() {
  bosh \
    -n \
    -t "${1}" \
    -d "${2}" \
    deploy
}

function upload_stemcell() {
  wget --quiet 'https://bosh.io/d/stemcells/bosh-softlayer-xen-ubuntu-trusty-go_agent' --output-document=stemcell.tgz
  bosh upload stemcell stemcell.tgz --skip-if-exists
}

function upload_remote_release() {
  local release_url=$1
  wget --quiet "${release_url}" -O remote_release.tgz
  bosh upload release remote_release.tgz
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
  default_env:
    bosh:
      password: ~
  cf_version: ${OLD_CF_RELEASE}
EOF
}

function main(){
  local root="${1}"
	set +x
  bosh target https://${BOSH_DIRECTOR}:25555
  bosh login ${BOSH_USER} ${BOSH_PASSWORD}
	set -x

  mkdir ${root}/stubs

  pushd ${root}/stubs
    generate_stub > data.yml
  popd

  spiff merge \
    "${root}/postgres-ci-env/deployments/cf/pgci-cf${OLD_CF_RELEASE}.yml" \
    "${root}/stubs/data.yml" \
    "${root}/postgres-ci-env/deployments/common/common.yml" > "${root}/${CF_DEPLOYMENT}.yml"

  upload_stemcell
  upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/cf-release?v=${OLD_CF_RELEASE}"

  deploy \
    "${BOSH_DIRECTOR}" \
    "${root}/${CF_DEPLOYMENT}.yml"

}


main "${PWD}"
