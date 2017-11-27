#!/bin/bash -exu

# Cannot set -u before sourcing .bashrc because of
# all the unbound variables in things beyond our control.
set +u
source ~/.bashrc
set -u

preflight_check() {
  set +x
  test -n "${RELEASE_NAME}"
  set -x
}

function main() {
  local root_dir
  root_dir="${PWD}"
  preflight_check

  local release_name
  release_name="${1}"

  local master_branch
  master_branch="${2}"

  configure_bucket "${release_name}"
  create_and_commit "${root_dir}" "${release_name}" "${master_branch}"
  copy_to_output "${root_dir}"
}

function configure_bucket() {
  local release_name
  release_name="${1}"

  set +x
  ./release-repo/ci/scripts/configure_final_release_bucket "${release_name}" ./oss-s3-buckets-stack ./release-repo/config
  set -x
}

function create_and_commit() {
  local root_dir
  root_dir="${1}"

  local release_name
  release_name="${2}"

  local master_branch
  master_branch="${3}"

  pushd "${root_dir}/release-repo" > /dev/null
    git config user.name "CF MEGA BOT"
    git config user.email "cf-mega@pivotal.io"

    git remote add -f master-repo "${root_dir}/release-repo-master"
    git merge "master-repo/${master_branch}" -m 'Merge with master'

    local exit_status
    for i in {1..5}; do
      /opt/rubies/ruby-2.2.4/bin/bosh -n create release --with-tarball --final
      exit_status="${PIPESTATUS[0]}"

      if [[ "${exit_status}" == "0" ]]; then
        break
      fi
    done

    if [[ "${exit_status}" != "0" ]]; then
      echo "Failed to Create ${release_name} Release"
      exit "${exit_status}"
    fi

    local new_release_version
    new_release_version="$(find releases -regex ".*${release_name}-[0-9]*.yml" | egrep -o "${release_name}-[0-9]+" | egrep -o "[0-9]+" | sort -n | tail -n 1)"
	
    source jobs/postgres/templates/pgconfig.sh.erb
    sed -i "/^versions:/a\ \ ${new_release_version}: \"PostgreSQL ${current_version}\"" versions.yml
    git add versions.yml

    git add .final_builds releases
    git commit -m "Final release ${new_release_version}"

    echo "${new_release_version}" > version_number
  popd > /dev/null
}

function copy_to_output() {
  local root_dir
  root_dir="${1}"

  shopt -s dotglob
  cp -R "${root_dir}/release-repo/"* "${root_dir}/final-release-repo"
}

main "${RELEASE_NAME}" "${MASTER_BRANCH:-"master"}"
