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

function upload_remote_release() {
  local release_url=$1
  wget --quiet "${release_url}" -O remote_release.tgz
  bosh upload release remote_release.tgz
}

generate_releases_stub() {
  local build_dir
  build_dir="${1}"

  cat <<EOF
---
releases:
- name: cf
  version: create
  url: file://${build_dir}/cf-release
- name: postgres
  version: create
  url: file://${build_dir}/postgres-release
EOF
}

generate_releases_stub_test() {
  local build_dir
  build_dir="${1}"

  cat <<EOF
---
releases:
- name: cf
  version: latest
- name: postgres
  version: create
  url: file://${build_dir}/postgres-release
EOF
}

generate_stemcell_stub() {
  pushd /tmp > /dev/null
    curl -Ls -o /dev/null -w %{url_effective} https://bosh.io/d/stemcells/bosh-softlayer-xen-ubuntu-trusty-go_agent | xargs -n 1 curl -O
  popd > /dev/null

  local stemcell_filename
  stemcell_filename=$(echo /tmp/light-bosh-stemcell-*-softlayer-xen-ubuntu-trusty-go_agent.tgz)

  local stemcell_version
  stemcell_version=$(echo ${stemcell_filename} | cut -d "-" -f4)

  cat <<EOF
---
meta:
  stemcell:
    name: bosh-softlayer-xen-ubuntu-trusty-go_agent
    version: ${stemcell_version}
    url: file://${stemcell_filename}
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
  local haproxy_instances
  vm_prefix="${CF_DEPLOYMENT}-"
  apps_domain="apps.${CF_DEPLOYMENT}.microbosh"
  cf1_domain="cf1.${CF_DEPLOYMENT}.microbosh"
  if [ "$HAPROXY_DEPLOYMENT" != "none" ]; then
    haproxy_instances=0
  else
    haproxy_instances=1
  fi
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
  haproxy_instances: ${haproxy_instances}
  default_env:
    bosh:
      password: ~
EOF
}

function main(){
  local root="${1}"

  set +x
  bosh target https://${BOSH_DIRECTOR}:25555
  bosh login ${BOSH_USER} ${BOSH_PASSWORD}
  set -x
  mkdir stubs

  #upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/cf-release"
  pushd stubs
    #generate_releases_stub_test ${root} > releases.yml
    generate_releases_stub ${root} > releases.yml
    generate_stemcell_stub > stemcells.yml
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
      "${root}/stubs/stemcells.yml" \
      "${root}/stubs/job_templates.yml" \
      "${root}/partial-pgci-cf.yml" > "${root}/pgci_cf.yml"
  popd

  deploy \
    "${BOSH_DIRECTOR}" \
    "${root}/pgci_cf.yml"

  if [ "$HAPROXY_DEPLOYMENT" != "none" ]; then
    bosh -t $BOSH_DIRECTOR download manifest $HAPROXY_DEPLOYMENT ha_manifest.yml
    bosh -t ${BOSH_DIRECTOR} -d ha_manifest.yml -n restart ha_proxy 0
  fi
}


main "${PWD}"
