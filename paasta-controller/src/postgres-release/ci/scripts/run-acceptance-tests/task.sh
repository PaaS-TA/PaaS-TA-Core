#!/bin/bash -exu

root="${PWD}"

function create_config_file() {
  indented_cert=$(echo "$BOSH_CA_CERT" | awk '$0="    "$0')
  cat <<EOF
---
bosh:
  target: $BOSH_DIRECTOR
  username: $BOSH_CLIENT
  password: $BOSH_CLIENT_SECRET
  director_ca_cert: |+
$indented_cert
cloud_configs:
  default_vm_type: "pgats"
  default_persistent_disk_type: "dp_10G"
postgres_release_version: $REL_VERSION
postgresql_version: "PostgreSQL ${current_version}"
EOF
}

preflight_check() {
  set +x
  test -n "${BOSH_DIRECTOR}"
  test -n "${BOSH_CLIENT}"
  test -n "${BOSH_CLIENT_SECRET}"
  test -n "${BOSH_CA_CERT}"
  test -n "${REL_VERSION}"
  set -x
}

function main() {
  preflight_check
  source ${root}/postgres-release/jobs/postgres/templates/pgconfig.sh.erb
  config_file="${root}/pgats_config.yml"
  create_config_file > $config_file
  to_dir=${GOPATH}/src/github.com/cloudfoundry/postgres-release
  mkdir -p $to_dir
  cp -R ${root}/postgres-release/* $to_dir
  PGATS_CONFIG="$config_file" "$to_dir/src/acceptance-tests/scripts/test-minimal"
}

main
