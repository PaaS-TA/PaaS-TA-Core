#!/bin/bash -exu

export ROOT=${PWD}

function preflight_check() {
  set +x
  test -n "${BOSH_DIRECTOR}"
  test -n "${BOSH_USER}"
  test -n "${BOSH_PASSWORD}"
  test -n "${BAREMETAL_IP}"
  test -n "${SSH_KEY}"
  set -x
}

function setup_ssh() {
  set +x
  echo "${SSH_KEY}" > ${ROOT}/.ssh-key
  chmod 600 ${ROOT}/.ssh-key
  mkdir -p ~/.ssh && chmod 700 ~/.ssh

  ssh-keyscan -t rsa,dsa ${BAREMETAL_IP} >> ~/.ssh/known_hosts
  export SSH_CONNECTION_STRING="root@${BAREMETAL_IP} -i ${ROOT}/.ssh-key"
  set -x
}

function cleanup_boshlite() {
  set +x
  ssh ${SSH_CONNECTION_STRING} "bosh target https://${BOSH_DIRECTOR}:25555"
  ssh ${SSH_CONNECTION_STRING} "bosh login ${BOSH_USER} ${BOSH_PASSWORD}"
  ssh ${SSH_CONNECTION_STRING} "bosh -n delete deployment cf-warden"
  ssh ${SSH_CONNECTION_STRING} "bosh cleanup --all"
  ssh ${SSH_CONNECTION_STRING} "rm -rf /tmp/cloudfoundry"
  ssh ${SSH_CONNECTION_STRING} "rm -rf /tmp/cf*yml; rm -rf /tmp/cf*tgz"
  set -x
}

function main() {
  preflight_check

  setup_ssh
  cleanup_boshlite	
}

main
