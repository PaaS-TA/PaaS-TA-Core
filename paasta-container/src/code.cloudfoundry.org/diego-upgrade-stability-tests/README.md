# Diego Upgrade Stability Tests (DUSTs)

**Note**: This repository should be imported as `code.cloudfoundry.org/diego-upgrade-stability-tests`.

This test suite exercises the upgrade path from the stable CF/Diego configuration to a current CF/Diego configuration.

## Usage

### Upload the necessary legacy releases to your bosh-lite

```bash
# Legacy Releases
bosh upload release https://bosh.io/d/github.com/cloudfoundry/cf-release?v=224
bosh upload release https://bosh.io/d/github.com/cloudfoundry/diego-release?v=0.1440.0
bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/garden-linux-release?v=0.307.0

# Current Releases
bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/etcd-release
bosh upload release https://bosh.io/d/github.com/cloudfoundry/garden-runc-release
bosh upload release https://bosh.io/d/github.com/cloudfoundry/cflinuxfs2-rootfs-release

# Optional Releases
bosh upload release https://bosh.io/d/github.com/cloudfoundry/cf-mysql-release
```

### Checkout the correct version of legacy releases

The V0 manifest generation depends on having cf-release and diego-release cloned to an additional directory.
The desired versions of each release should be checked out.

```bash
cd ~/workspace/cf-release-v0
git checkout v220
git submodule update --init --recursive src/loggregator # Need manifest generation templates for LAMB
cd ~/workspace/diego-release-v0
git checkout 0.1434.0
```

### Upload the necessary V-prime releases to your bosh-lite

```bash
cd ~/workspace/cf-release
git checkout runtime-passed
./scripts/update
bosh -n --parallel 10 create release --force && bosh upload release
cd ~/workspace/diego-release
git checkout develop
./scripts/update
bosh -n --parallel 10 create release --force && bosh upload release
```

### Upload the necessary stemcell to your bosh-lite

```
bosh upload stemcell https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent
```

### Run the test suite

The DUSTs require the CONFIG environment variable to be set to the path of a valid configuration JSON file.
The following commands will setup the `CONFIG` for a [BOSH-Lite](https://github.com/cloudfoundry/bosh-lite) installation.
Replace credentials and URLs as appropriate for your environment.

```bash
cat > config.json <<EOF
{
  "override_domain": "bosh-lite.com",
  "bosh_director_url": "192.168.50.4",
  "bosh_admin_user": "admin",
  "bosh_admin_password": "admin",
  "base_directory": "$HOME/workspace/",
  "v0_cf_release_path": "[LEGACY CF RELEASE DIR]",
  "v0_diego_release_path": "[LEGACY DIEGO RELEASE DIR]",
  "v1_cf_release_path": "[CF RELEASE DIR]",
  "v1_diego_release_path": "[DIEGO RELEASE DIR]",
  "max_polling_errors": 1,
  "aws_stubs_directory": REPLACE_ME,
  "use_sql_vprime": false
}
EOF
export CONFIG=$PWD/config.json
```

Make sure the release directories for the legacy and latest Cloud Foundry and Diego are named `cf-release` and `diego-release`, as otherwise the deployments will fail.

BOSH-Lite deployments of CF v220 that use the 'local' blobstore cannot be
upgraded to CF deployments that use the `blobstore` job. To work around this,
the DUSTs must be configured to use an AWS S3 bucket as the CC blobstore. Create
a directory with stubs to configure that blobstore, then supply the path of that
directory as the value of the `aws_stubs_directory` configuration parameter.

The `use_sql_vprime` boolean property specifies whether the BBS should migrate
data from etcd to the SQL store. It should only set to `true` if the cf-mysql
release has been uploaded.

Run the test suite by invoking the ginkgo CLI from the root of this repository:

```bash
ginkgo -v
```
