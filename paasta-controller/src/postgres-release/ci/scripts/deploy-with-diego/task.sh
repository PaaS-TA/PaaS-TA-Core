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
  Bosh_ip: ${BOSH_DIRECTOR}
  Bosh_public_ip: ${BOSH_PUBLIC_IP}
  stemcell_version: ${STEMCELL_VERSION}
  default_env:
    bosh:
      password: ~
      keep_root_password: true
EOF
}

function deploy_diego() {
  bosh -d ${CF_DEPLOYMENT} manifest > ${root}/pgci_cf.yml

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

  spiff merge \
    "$root/postgres-ci-env/deployments/diego/sql_overrides.yml" \
    "${root}/env.yml" > "${root}/sql_overrides.yml"

  pushd diego-release > /dev/null
    ./scripts/generate-deployment-manifest \
      -c $root/pgci_cf.yml \
      -i $root/iaas-settings.yml \
      -p $root/property-overrides.yml \
      -n $root/postgres-ci-env/deployments/diego/instance-count-overrides.yml \
      -v $root/postgres-ci-env/deployments/diego/release-versions.yml \
      -s $root/sql_overrides.yml \
      -x \
      -r \
      > $root/pgci_diego.yml

  popd > /dev/null

  bosh -n deploy -d "${CF_DEPLOYMENT}-diego" "$root/pgci_diego.yml"
}

function upload_release() {
  local release
  release=${1}
  bosh upload-release https://bosh.io/d/github.com/${release}
}

preflight_check() {
  set +x
  test -n "${BOSH_DIRECTOR}"
  test -n "${BOSH_PUBLIC_IP}"
  test -n "${BOSH_CLIENT}"
  test -n "${BOSH_CLIENT_SECRET}"
  test -n "${BOSH_CA_CERT}"
  test -n "${CF_DEPLOYMENT}"
  test -n "${API_USER}"
  test -n "${API_PASSWORD}"
  test -n "${STEMCELL_VERSION}"
  set -x
}

function main() {
  preflight_check
  export BOSH_ENVIRONMENT="https://${BOSH_DIRECTOR}:25555"
  upload_release "cloudfoundry/cflinuxfs2-release"
  upload_release "cloudfoundry/diego-release"
  upload_release "cloudfoundry/garden-runc-release"

  deploy_diego
}

main
