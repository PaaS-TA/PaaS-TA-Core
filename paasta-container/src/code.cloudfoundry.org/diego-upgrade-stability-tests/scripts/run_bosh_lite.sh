#!/usr/bin/env bash

BOSH_USER=admin
BOSH_PASSWORD=admin
CF_RELEASE_V0_SHA=247
CF_RELEASE_V1_SHA=release-candidate
DIEGO_RELEASE_V0_SHA=1.0.0
GARDEN_RUNC_RELEASE_V0_SHA=1.0.3
CFLINUXFS2_RELEASE_V0_SHA=1.40.0
CF_MYSQL_RELEASE_SHA=35
STEMCELL_VERSION=3421.9

base_directory=/tmp/base_directory

function clone {
    if [ $# -ne 3 ]; then
        echo "Usage: $0 repo-url directory sha"
        exit 1
    fi
    if [ ! -d $2 ]; then
        git clone $1 $2
    fi
    pushd $2
        git co $3
        git su
    popd
}

function upload_release {
    if [ $# -ne 1 ]; then
        echo "usage $0 release-dir"
        exit 1
    fi
    pushd $1
      rbosh reset release
      rbosh -n create release --force && rbosh -n upload release
    popd
}

function upload_bosh_io_release {
    if [ $# -ne 1]; then
        echo "usage $0 release-dir"
        exit 1
    fi
    release=$(mktemp releaseXXXX.tgz)
    url=$1
    wget -O $release $url
    rbosh upload release $release && rm $release
}

function upload_bosh_io_stemcell {
    if [ $# -ne 1]; then
        echo "usage $0 release-dir"
        exit 1
    fi
    release=$(mktemp releaseXXXX.tgz)
    url=$1
    wget -O $release $url
    rbosh upload stemcell $release && rm $release
}

mkdir -p $base_directory
pushd $base_directory
  upload_bosh_io_stemcell https://s3.amazonaws.com/bosh-core-stemcells/warden/bosh-stemcell-$STEMCELL_VERSION-warden-boshlite-ubuntu-trusty-go_agent.tgz
  upload_bosh_io_release http://bosh.io/d/github.com/cloudfoundry/cf-release?v=$CF_RELEASE_V0_SHA
  upload_bosh_io_release http://bosh.io/d/github.com/cloudfoundry/diego-release?v=$DIEGO_RELEASE_V0_SHA
  upload_bosh_io_release http://bosh.io/d/github.com/cloudfoundry/garden-runc-release?v=$GARDEN_RUNC_RELEASE_V0_SHA
  upload_bosh_io_release http://bosh.io/d/github.com/cloudfoundry/cflinuxfs2-rootfs-release?v=$CFLINUXFS2_RELEASE_V0_SHA
  upload_bosh_io_release http://bosh.io/d/github.com/cloudfoundry/garden-runc-release
  upload_bosh_io_release http://bosh.io/d/github.com/cloudfoundry/cflinuxfs2-release
  upload_bosh_io_release http://bosh.io/d/github.com/cloudfoundry/cf-mysql-release?v=$CF_MYSQL_RELEASE_SHA
  upload_bosh_io_release http://bosh.io/d/github.com/cloudfoundry-incubator/cf-routing-release

  clone https://github.com/cloudfoundry/cf-release cf-stable-release v$CF_RELEASE_V0_SHA
  clone https://github.com/cloudfoundry/cf-release cf-release $CF_RELEASE_V1_SHA

  clone https://github.com/cloudfoundry/diego-release diego-stable-release v$DIEGO_RELEASE_V0_SHA

  ln -sf $HOME/workspace/diego-release
  ln -sf $HOME/workspace/deployments-diego

  upload_release cf-release
  upload_release diego-release
popd

mkdir -p $base_directory/bin
pushd $base_directory/bin
    ln -sf `which rbosh` bosh
popd
export PATH=$base_directory/bin:$PATH

cat > $base_directory/dusts_config.json <<EOF
{
  "bosh_director_url": "192.168.50.4",
  "bosh_admin_user": "${BOSH_USER}",
  "bosh_admin_password": "${BOSH_PASSWORD}",
  "base_directory": "${base_directory}",
  "v0_cf_release_path": "cf-stable-release",
  "v0_diego_release_path": "diego-stable-release",
  "v1_cf_release_path": "cf-release",
  "v1_diego_release_path": "diego-release",
  "override_domain": "bosh-lite.com",
  "max_polling_errors": 1,
  "aws_stubs_directory": "deployments-diego/diego-ci/stubs/cf",
  "use_sql_v0": true,
  "diego_release_v0_legacy": true
}
EOF

pushd $base_directory/diego-release/src/code.cloudfoundry.org/diego-upgrade-stability-tests
    export CONFIG=${base_directory}/dusts_config.json
    export BOSH_LITE_PASSWORD=${BOSH_PASSWORD}

    ginkgo -v
popd
