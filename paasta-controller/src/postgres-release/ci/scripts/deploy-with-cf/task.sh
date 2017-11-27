#!/bin/bash -eu

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
  test -n "${REL_VERSION}"
  test -n "${STEMCELL_VERSION}"
  set -x
}

generate_releases_stub() {
  local build_dir
  build_dir="${1}"

  cat <<EOF
---
releases:
- name: cf
  version: ${REL_VERSION}
- name: postgres
  version: ${REL_VERSION}
EOF
}

generate_job_templates_stub() {
  cat <<EOF
meta:
  <<: (( merge ))
  postgres_templates:
  - name: postgres
    release: postgres
EOF
}

generate_env_stub() {
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
EOF
}

function main(){
  local root="${1}"
  preflight_check

  export BOSH_ENVIRONMENT="https://${BOSH_DIRECTOR}:25555"
  mkdir stubs

  pushd stubs
    generate_releases_stub ${root} > releases.yml
    generate_job_templates_stub > job_templates.yml
    generate_env_stub > env.yml
  popd

  pushd "${root}/cf-release"
    spiff merge \
      "${root}/postgres-ci-env/deployments/cf/pgci-cf.yml" \
      "${root}/postgres-ci-env/deployments/common/properties.yml" \
      "${root}/stubs/env.yml" \
      "${root}/postgres-ci-env/deployments/common/common.yml" > "${root}/partial-pgci-cf.yml"

    spiff merge \
      "templates/generic-manifest-mask.yml" \
      "templates/cf.yml" \
      "${root}/postgres-ci-env/deployments/cf/cf-infrastructure-softlayer.yml" \
      "${root}/stubs/releases.yml" \
      "${root}/stubs/job_templates.yml" \
      "${root}/partial-pgci-cf.yml" > "${root}/pgci_cf.yml"
  popd

  bosh -n deploy -d "${CF_DEPLOYMENT}" "${root}/pgci_cf.yml"
}

main "${PWD}"
