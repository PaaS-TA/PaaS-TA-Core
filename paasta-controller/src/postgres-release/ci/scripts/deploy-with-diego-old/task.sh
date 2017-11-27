#!/bin/bash -eu

preflight_check() {
  set +x
  test -n "${BOSH_DIRECTOR}"
  test -n "${BOSH_PUBLIC_IP}"
  test -n "${BOSH_CLIENT}"
  test -n "${BOSH_CLIENT_SECRET}"
  test -n "${BOSH_CA_CERT}"
  test -n "${OLD_CF_RELEASE}"
  test -n "${OLD_DIEGO_RELEASE}"
  test -n "${OLD_GARDEN_RELEASE}"
  test -n "${OLD_ETCD_RELEASE}"
  test -n "${OLD_ROOTFS_RELEASE}"
  test -n "${CF_DEPLOYMENT}"
  test -n "${DIEGO_DEPLOYMENT}"
  test -n "${OLD_STEMCELL}"
  test -n "${STEMCELL_VERSION}"
  set -x
}

function upload_stemcell() {
  if [ "${OLD_STEMCELL}" != "latest" ]; then
    local old_stemcell_url="https://s3.amazonaws.com/bosh-softlayer-cpi-stemcells/light-bosh-stemcell-${OLD_STEMCELL}-softlayer-esxi-ubuntu-trusty-go_agent.tgz"
    wget --quiet "${old_stemcell_url}" --output-document=stemcell.tgz
    bosh upload-stemcell stemcell.tgz
    OLD_STEMCELL_NAME=bosh-softlayer-esxi-ubuntu-trusty-go_agent
  else
    OLD_STEMCELL_NAME=bosh-softlayer-xen-ubuntu-trusty-go_agent
  fi
}

function upload_remote_release() {
  local release_url=$1
  wget --quiet "${release_url}" -O remote_release.tgz
  bosh upload-release remote_release.tgz
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
  Bosh_ip: ${BOSH_DIRECTOR}
  Bosh_public_ip: ${BOSH_PUBLIC_IP}
  stemcell_version: ${STEMCELL_VERSION}
  cell_stemcell:
    name: ${OLD_STEMCELL_NAME}
    version: ${OLD_STEMCELL}
  default_env:
    bosh:
      password: ~
      keep_root_password: true
  diego_version: ${OLD_DIEGO_RELEASE}
  garden_version: ${OLD_GARDEN_RELEASE}
  etcd_version: ${OLD_ETCD_RELEASE}
  cf_version: ${OLD_CF_RELEASE}
  rootfs_version: ${OLD_ROOTFS_RELEASE}
EOF
}

function main(){
  local root="${1}"
  preflight_check
  export BOSH_ENVIRONMENT="https://${BOSH_DIRECTOR}:25555"
  OLD_STEMCELL_NAME=bosh-softlayer-xen-ubuntu-trusty-go_agent
  upload_stemcell

  mkdir ${root}/stubs

  pushd ${root}/stubs
    generate_meta_stub > meta.yml
  popd

  spiff merge \
    "${root}/postgres-ci-env/deployments/diego/pgci-diego${OLD_CF_RELEASE}.yml" \
    "${root}/postgres-ci-env/deployments/common/properties.yml" \
    "${root}/stubs/meta.yml" \
    "${root}/postgres-ci-env/deployments/common/common.yml" > "${root}/${DIEGO_DEPLOYMENT}.yml"

  upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/diego-release?v=${OLD_DIEGO_RELEASE}"
  upload_remote_release "https://bosh.io/d/github.com/cloudfoundry-incubator/etcd-release?v=${OLD_ETCD_RELEASE}"
  if [ "${OLD_CF_RELEASE}" -gt "257" ]; then
    upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/garden-runc-release?v=${OLD_GARDEN_RELEASE}"
    upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/cflinuxfs2-release?v=${OLD_ROOTFS_RELEASE}"
  elif [ "${OLD_CF_RELEASE}" -gt "250" ]; then
    upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/garden-runc-release?v=${OLD_GARDEN_RELEASE}"
    upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/cflinuxfs2-rootfs-release?v=${OLD_ROOTFS_RELEASE}"
  else
    upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/garden-linux-release?v=${OLD_GARDEN_RELEASE}"
  fi

  bosh -n deploy -d "${DIEGO_DEPLOYMENT}" "${root}/${DIEGO_DEPLOYMENT}.yml"
}


main "${PWD}"
