#!/bin/bash -exu

export ROOT=${PWD}

function preflight_check() {
  set +x
  test -n "${BOSH_DIRECTOR}"
  test -n "${BOSH_USER}"
  test -n "${BOSH_PASSWORD}"
  test -n "${BAREMETAL_IP}"
  test -n "${SSH_KEY}"
  test -n "${OLD_CF_RELEASE}"
  set -x
}

function setup_ssh() {
  set +x
  echo "${SSH_KEY}" > ${ROOT}/.ssh-key
  chmod 600 ${ROOT}/.ssh-key
  mkdir -p ~/.ssh && chmod 700 ~/.ssh

  ssh-keyscan -t rsa,dsa ${BAREMETAL_IP} >> ~/.ssh/known_hosts
  export SSH_CONNECTION_STRING="root@${BAREMETAL_IP} -i ${ROOT}/.ssh-key"
  export SCP_CONN="-i ${PWD}/.ssh-key root@${BAREMETAL_IP}"
  set -x
}

function setup_boshlite() {
	set +x
  ssh ${SSH_CONNECTION_STRING} "bosh target https://${BOSH_DIRECTOR}:25555"
  ssh ${SSH_CONNECTION_STRING} "bosh login ${BOSH_USER} ${BOSH_PASSWORD}"
	set -x

  upload_stemcell
  upload_remote_release "https://bosh.io/d/github.com/cloudfoundry/cf-release?v=${OLD_CF_RELEASE}"
}

function upload_stemcell() {
  ssh ${SSH_CONNECTION_STRING} "wget --quiet 'https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent' --output-document=stemcell.tgz"
  ssh ${SSH_CONNECTION_STRING} "bosh upload stemcell stemcell.tgz"
}

function upload_remote_release() {
  local release_url=$1
  ssh ${SSH_CONNECTION_STRING} "wget --quiet '${release_url}' -O remote_release.tgz"
  ssh ${SSH_CONNECTION_STRING} "bosh upload release remote_release.tgz"
}

function deploy_release() {
  scp -i ${ROOT}/.ssh-key ${ROOT}/postgres-ci-env/deployments/boshlite/cf-${OLD_CF_RELEASE}.yml root@${BAREMETAL_IP}:/tmp
  ssh ${SSH_CONNECTION_STRING} "bosh -d /tmp/cf-${OLD_CF_RELEASE}.yml -n deploy"
}

function generate_latest_manifest() {
  cat <<EOF > ${ROOT}/releases.yml
---
releases:
- name: cf
  version: create
  url: file:///tmp/cloudfoundry/cf-release
- name: postgres
  version: create
  url: file:///tmp/cloudfoundry/postgres-release
EOF

  cat <<EOF > ${ROOT}/job_templates.yml
meta:
  <<: (( merge ))
  postgres_templates:
  - name: postgres
    release: postgres
EOF
   cat <<EOF > ${ROOT}/director_uuid.yml
director_uuid: 28539132-6d43-4e1b-bf40-f2ce032ee9f8
EOF

  ${ROOT}/cf-release/scripts/generate_deployment_manifest bosh-lite \
    "${ROOT}/director_uuid.yml" \
    "${ROOT}/job_templates.yml" \
    "${ROOT}/releases.yml" \
    "${ROOT}/postgres-ci-env/deployments/boshlite/fog.yml" > "${ROOT}/cf-latest.yml"
  echo YML name is ${ROOT}/cf-latest.yml
  scp -i ${ROOT}/.ssh-key ${ROOT}/cf-latest.yml root@${BAREMETAL_IP}:/tmp
}

function upload_latest_and_deploy() {
  pushd ${ROOT}
  tar czf cf-postgres.tgz postgres-release cf-release

  scp -i ./.ssh-key cf-postgres.tgz root@${BAREMETAL_IP}:/tmp
  popd
  ssh ${SSH_CONNECTION_STRING} "mkdir -p /tmp/cloudfoundry; cd /tmp/cloudfoundry; tar xf /tmp/cf-postgres.tgz"
  ssh ${SSH_CONNECTION_STRING} "bosh -d /tmp/cf-latest.yml -n deploy"
}

function main() {
	preflight_check

	setup_ssh
	setup_boshlite	

	deploy_release
  generate_latest_manifest
  upload_latest_and_deploy
}

main
