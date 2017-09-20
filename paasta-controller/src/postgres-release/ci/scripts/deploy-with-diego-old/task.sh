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
  local old_stemcell_url="https://s3.amazonaws.com/bosh-softlayer-cpi-stemcells/light-bosh-stemcell-${OLD_STEMCELL}-softlayer-esxi-ubuntu-trusty-go_agent.tgz"
  wget --quiet 'https://bosh.io/d/stemcells/bosh-softlayer-xen-ubuntu-trusty-go_agent' --output-document=stemcell.tgz
  bosh upload stemcell stemcell.tgz --skip-if-exists
  wget --quiet "${old_stemcell_url}" --output-document=stemcell.tgz
  bosh upload stemcell stemcell.tgz --skip-if-exists
}

function upload_remote_release() {
  local release_url=$1
  wget --quiet "${release_url}" -O remote_release.tgz
  bosh upload release remote_release.tgz
}

generate_meta_stub() {
  local vm_prefix
  local cf1_domain
  local apps_domain
  vm_prefix="${DIEGO_DEPLOYMENT}-"
  apps_domain="apps.${CF_DEPLOYMENT}.microbosh"
  cf1_domain="cf1.${CF_DEPLOYMENT}.microbosh"
  cat <<EOF
---
common_data:
  <<: (( merge ))
  VmNamePrefix: ${vm_prefix}
  cf1_domain: ${cf1_domain}
  env_name: ${DIEGO_DEPLOYMENT}
  apps_domain: ${apps_domain}
  cell_stemcell:
    name: bosh-softlayer-esxi-ubuntu-trusty-go_agent
    version: ${OLD_STEMCELL}
  default_env:
    bosh:
      password: ~
  diego_version: ${OLD_DIEGO_RELEASE}
  garden_version: ${OLD_GARDEN_RELEASE}
  etcd_version: ${OLD_ETCD_RELEASE}
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
    generate_meta_stub > meta.yml
  popd

  spiff merge \
    "${root}/postgres-ci-env/deployments/diego/pgci-diego${OLD_CF_RELEASE}.yml" \
    "${root}/stubs/meta.yml" \
    "${root}/postgres-ci-env/deployments/common/common.yml" > "${root}/${DIEGO_DEPLOYMENT}.yml"

  upload_stemcell
  upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/diego-release?v=${OLD_DIEGO_RELEASE}"
  upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/garden-linux-release?v=${OLD_GARDEN_RELEASE}"
  upload_remote_release "https://bosh.io/d/github.com/cloudfoundry-incubator/etcd-release?v=${OLD_ETCD_RELEASE}"

  deploy \
    "${BOSH_DIRECTOR}" \
    "${root}/${DIEGO_DEPLOYMENT}.yml"

}


main "${PWD}"
