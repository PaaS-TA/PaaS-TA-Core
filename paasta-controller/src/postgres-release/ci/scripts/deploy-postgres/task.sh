#!/bin/bash -eu

preflight_check() {
  set +x
  test -n "${BOSH_DIRECTOR}"
  test -n "${BOSH_PUBLIC_IP}"
  test -n "${BOSH_CLIENT}"
  test -n "${BOSH_CLIENT_SECRET}"
  test -n "${BOSH_CA_CERT}"
  test -n "${PG_DEPLOYMENT}"
  test -n "${PG_VERSION}"
  test -n "${PG_USER}"
  test -n "${PG_PSW}"
  test -n "${PG_PORT}"
  test -n "${PG_DB}"
  test -n "${STEMCELL_VERSION}"
  set -x
}

upload_remote_release() {
  local release_url=$1
  wget --quiet "${release_url}" -O remote_release.tgz
  bosh upload-release remote_release.tgz
}

generate_dev_release_stub() {
  local build_dir
  build_dir="${1}"

  cat <<EOF
---
releases:
- name: postgres
  version: create
  url: file://${build_dir}
EOF
}

generate_uploaded_release_stub() {
  local release_version
  release_version="${1}"

  cat <<EOF
---
releases:
- name: postgres
  version: ${release_version}
EOF
}

generate_env_stub() {
  local vm_prefix
  local hostname
  vm_prefix="${PG_DEPLOYMENT}-"
  hostname="0.postgres.default.${PG_DEPLOYMENT}.microbosh"
  set +x
  cat <<EOF
---
common_data:
  <<: (( merge ))
  VmNamePrefix: ${vm_prefix}
  env_name: ${PG_DEPLOYMENT}
  pg_user: ${PG_USER}
  pg_password: ${PG_PSW}
  pg_db: ${PG_DB}
  pg_port: ${PG_PORT}
  pg_host: ${hostname}
  Bosh_ip: ${BOSH_DIRECTOR}
  Bosh_public_ip: ${BOSH_PUBLIC_IP}
  stemcell_version: ${STEMCELL_VERSION}
EOF
  set -x
}

function main(){
  local root="${1}"
  preflight_check
  export BOSH_ENVIRONMENT="https://${BOSH_DIRECTOR}:25555"

  mkdir stubs

  pushd stubs
    if [ "${PG_VERSION}" == "master" ]; then
      generate_dev_release_stub ${root}/postgres-release-master > releases.yml
    elif [ "${PG_VERSION}" == "develop" ]; then
      generate_dev_release_stub ${root}/postgres-release > releases.yml
    else
      upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/postgres-release?v=${PG_VERSION}"
      generate_uploaded_release_stub ${PG_VERSION} > releases.yml
    fi
    generate_env_stub > env.yml
  popd

  pushd "${root}/postgres-release"
    spiff merge \
      "${root}/postgres-ci-env/deployments/postgres/iaas-infrastructure.yml" \
      "${root}/stubs/env.yml" \
      "${root}/postgres-ci-env/deployments/common/common.yml" > "${root}/iaas.yml"
    spiff merge \
      "${root}/postgres-ci-env/deployments/postgres/properties.yml" \
      "${root}/stubs/env.yml" \
      "${root}/postgres-ci-env/deployments/common/common.yml" > "${root}/props.yml"
    scripts/generate-deployment-manifest \
      -i "${root}/iaas.yml" \
      -p "${root}/props.yml" \
      -v "${root}/stubs/releases.yml" > "${root}/pgci-postgres.yml"
  popd

  bosh -n deploy -d "${PG_DEPLOYMENT}" "${root}/pgci-postgres.yml"
}


main "${PWD}"
