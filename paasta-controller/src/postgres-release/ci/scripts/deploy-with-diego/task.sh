#!/bin/bash -exu

root="${PWD}"

generate_env_stub() {
  local vm_prefix
  local cf1_domain
  local apps_domain
  local diego_deployment

  vm_prefix="${CF_DEPLOYMENT}-diego-"
  apps_domain="apps.${CF_DEPLOYMENT}.microbosh"
  cf1_domain="cf1.${CF_DEPLOYMENT}.microbosh"
  diego_deployment="${CF_DEPLOYMENT}-diego"
  cat <<EOF
---
common_data:
  <<: (( merge ))
  VmNamePrefix: ${vm_prefix}
  cf1_domain: ${cf1_domain}
  env_name: ${diego_deployment}
  apps_domain: ${apps_domain}
  api_user: ${API_USER}
  api_password: ${API_PASSWORD}
  default_env:
    bosh:
      password: ~
EOF
}

function deploy_diego() {
  bosh -t $BOSH_DIRECTOR download manifest ${CF_DEPLOYMENT} ${root}/pgci_cf.yml

  generate_env_stub > env.yml

  spiff merge \
    "${root}/postgres-ci-env/deployments/diego/property-overrides.yml" \
    "${root}/env.yml" \
    "${root}/postgres-ci-env/deployments/common/common.yml" > "${root}/property-overrides.yml"

  spiff merge \
    "${root}/postgres-ci-env/deployments/diego/iaas-settings.yml" \
    "${root}/pgci_cf.yml" \
    "${root}/env.yml" \
    "${root}/postgres-ci-env/deployments/common/common.yml" > "${root}/iaas-settings.yml"

  pushd diego-release > /dev/null
    ./scripts/generate-deployment-manifest \
      -c $root/pgci_cf.yml \
      -i $root/iaas-settings.yml \
      -p $root/property-overrides.yml \
      -n $root/postgres-ci-env/deployments/diego/instance-count-overrides.yml \
      -v $root/postgres-ci-env/deployments/diego/release-versions.yml \
      > $root/pgci_diego.yml

  popd > /dev/null

  bosh -n \
    -d pgci_diego.yml \
    -t ${BOSH_DIRECTOR} \
    deploy
}

function upload_release() {
  local release
  release=${1}
  bosh -t ${BOSH_DIRECTOR} upload release https://bosh.io/d/github.com/${release}
}

function main() {
  upload_release "cloudfoundry/cflinuxfs2-rootfs-release"
  upload_release "cloudfoundry/diego-release"
  upload_release "cloudfoundry/garden-linux-release"
  upload_release "cloudfoundry-incubator/etcd-release"

  deploy_diego
}

main
