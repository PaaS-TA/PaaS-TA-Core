# etcd-release

This is a [BOSH](http://bosh.io) release for [etcd](https://github.com/coreos/etcd).

* [CI](https://mega.ci.cf-app.com/pipelines/etcd)
* [Roadmap](https://www.pivotaltracker.com/n/projects/1382120)

###Contents

* [Using Etcd](#using-etcd)
* [Deploying](#deploying)
* [Contributing](#contributing)
* [Running Tests](#running-tests)
* [Encryption](#encryption)
* [Failure Recovery](#failure-recovery)

## Using Etcd

Etcd is a distributed key-value store. It uses the
[Raft consensus algorithm](https://raft.github.io/) to manage fault tolerance.
Etcd provides a JSON HTTP API for managing the key-value set. The client
also provides an optional SSL-cert authentication mechanism.

### Within CloudFoundry

The primary use case for Etcd within CloudFoundry is as a cache for information
about where and how processes are running within the container runtime. It is
also used, to a much lesser extent, as a discovery mechanism for some components.

### Fault Tolerance and Data Durability

Etcd is a distributed computing system which conforms to the properties outlined
in the [CAP Theorem]( https://en.wikipedia.org/wiki/CAP_theorem). The theorem
outlines that such a system will have to make a tradeoff with regards to the
guarantees of consistency, availability, and partition tolerance. In the default
configuration, Etcd has a preference for availability and partition tolerance.
This means that under a network partition, it is possible to read "stale" data
from the cluster. This behavior only affects reads. Writes will still require
quorum to commit. Etcd has an optional query parameter that can be provided when
submitting read requests to force the read to be consistent. More information
about the fault tolerance profile of Etcd can be found in the
[documentation](https://coreos.com/etcd/docs/latest://coreos.com/etcd/docs/latest/).
Etcd was also investigated using the Jepsen framework. The results for that
investigation can be found
[here](https://aphyr.com/posts/316-jepsen-etcd-and-consul).

## Deploying

In order to deploy etcd-release you must follow the standard steps for deploying software with BOSH.

We assume you have already deployed and targeted a BOSH director. For more instructions on how to do that please see the [BOSH documentation](http://bosh.io/docs).

###1. Uploading a stemcell
Find the "BOSH Lite Warden" stemcell you wish to use. [bosh.io](https://bosh.io/stemcells) provides a resource to find and download stemcells.  Then run `bosh upload stemcell STEMCELL_URL_OR_PATH_TO_DOWNLOADED_STEMCELL`.

###2. Creating a release
From within the etcd-release director run `bosh create release --force` to create a development release.

###3. Uploading a release
Once you've created a development release run `bosh upload release` to upload your development release to the director.

### 4. Using a sample deployment manifest

We provide a set of sample deployment manifests that can be used as a starting point for creating your own manifest, but they should not be considered comprehensive. They are located in manifests/aws and manifests/bosh-lite.

###5. Deploy

Run `bosh -d OUTPUT_MANIFEST_PATH deploy`.

## Contributing

### Contributor License Agreement

Contributors must sign the Contributor License Agreement before their
contributions can be merged. Follow the directions
[here](https://www.cloudfoundry.org/community/contribute/) to complete
that process.

### Developer Workflow

Make sure that you are working against the `develop` branch. PRs submitted
against other branches will need to be resubmitted with the correct branch
targeted.

Before submitting a PR, make sure to run the test suites. Information about
how to run the suites can be seen in the [Running Tests](#running-tests)
section.

## Running Tests

We have written a test suite that exercises spinning up single/multiple etcd instances, scaling them
and perform rolling deploys. If you have already installed Go, you can run `EATS_CONFIG=[config_file.json] ./scripts/test`.
The `test` script installs all dependancies and runs the full test suite. The EATS_CONFIG
environment variable points to a configuration file which specifies the endpoint of the BOSH
director and the path to your iaas_settings stub. An example config json for BOSH-lite would look like:

```
cat > integration_config.json << EOF
{
  "bosh":{
    "target": "192.168.50.4",
    "username": "admin",
    "password": "admin"
  }
}
EOF
EATS_CONFIG=$PWD/integration_config.json ./scripts/test
```

The full set of config parameters is explained below:
* `bosh.target` (required) Public BOSH IP address that will be used to host test environment
* `bosh.username` (required) Username for the BOSH director login
* `bosh.password` (required) Password for the BOSH director login
* `bosh.director_ca_cert` BOSH Director CA Cert
* `aws.subnet` Subnet ID for AWS deployments
* `aws.access_key_id` Key ID for AWS deployments
* `aws.secret_access_key` Secret Access Key for AWS deployments
* `aws.default_key_name` Default Key Name for AWS deployments
* `aws.default_security_groups` Security groups for AWS deployments
* `aws.region` Region for AWS deployments
* `registry.host` Host for the BOSH registry
* `registry.port` Port for the BOSH registry
* `registry.username` Username for the BOSH registry
* `registry.password` Password for the BOSH registry

### Running acceptance tests as an errand

The acceptance tests can also be run as a bosh errand. An example manifest is included to run against bosh-lite.

```
bosh upload stemcell https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent --skip-if-exists
bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/consul-release
bosh upload release https://bosh.io/d/github.com/cppforlife/bosh-warden-cpi-release
bosh upload release https://bosh.io/d/github.com/cppforlife/turbulence-release
bosh create release --force
bosh upload release
bosh update cloud-config manifests/bosh-lite/cloud-config.yml
bosh -n -d manifests/bosh-lite/eats.yml deploy
bosh -d manifests/bosh-lite/eats.yml run errand acceptance-tests
```

## Encryption

### Encrypting Traffic

To force communication between clients and etcd to use SSL, enable the etcd.require_ssl manifest property to true.

To force communication between etcd nodes to use SSL, set the `etcd.peer_require_ssl` manifest property to true.

The instructions below detail how to create certificates. If SSL is required for client communication, the clients will also need copies of the certificates.

When either SSL option is enabled, communication to the etcd nodes is done by consul DNS addresses rather than by IP address. When SSL is disabled, IP addresses are used and consul is not a dependency.

### Generating SSL Certificates

For generating SSL certificates, we recommend [certstrap](https://github.com/square/certstrap).
An operator can follow the following steps to successfully generate the required certificates.

> Most of these commands can be found in [scripts/generate-certs](scripts/generate-certs)

1. Get certstrap
   ```
   go get github.com/square/certstrap
   cd $GOPATH/src/github.com/square/certstrap
   ./build
   cd bin
   ```

2. Initialize a new certificate authority.
   ```
   $ ./certstrap init --common-name "etcdCA"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/etcdCA.key
   Created out/etcdCA.crt
   ```

   The manifest property `properties.etcd.ca_cert` should be set to the certificate in `out/etcdCA.crt`

3. Create and sign a certificate for the etcd server.
   ```
   $ ./certstrap request-cert --common-name "etcd.service.consul" --domain "*.etcd.service.consul,etcd.service.consul"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/etcd.service.consul.key
   Created out/etcd.service.consul.csr

   $ ./certstrap sign etcd.service.consul --CA etcdCA
   Created out/etcd.service.consul.crt from out/etcd.service.consul.csr signed by out/etcdCA.key
   ```

   The manifest property `properties.etcd.server_cert` should be set to the certificate in `out/etcd.service.consul.crt`
   The manifest property `properties.etcd.server_key` should be set to the certificate in `out/etcd.service.consul.key`

4. Create and sign a certificate for etcd clients.
   ```
   $ ./certstrap request-cert --common-name "clientName"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/clientName.key
   Created out/clientName.csr

   $ ./certstrap sign clientName --CA etcdCA
   Created out/clientName.crt from out/clientName.csr signed by out/etcdCA.key
   ```

   The manifest property `properties.etcd.client_cert` should be set to the certificate in `out/clientName.crt`
   The manifest property `properties.etcd.client_key` should be set to the certificate in `out/clientName.key`

5. Initialize a new peer certificate authority. [optional]
   ```
   $ ./certstrap --depot-path peer init --common-name "peerCA"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created peer/peerCA.key
   Created peer/peerCA.crt
   ```

   The manifest property `properties.etcd.peer_ca_cert` should be set to the certificate in `peer/peerCA.crt`

6. Create and sign a certificate for the etcd peers. [optional]
   ```
   $ ./certstrap --depot-path peer request-cert --common-name "etcd.service.consul" --domain "*.etcd.service.consul,etcd.service.consul"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created peer/etcd.service.consul.key
   Created peer/etcd.service.consul.csr

   $ ./certstrap --depot-path peer sign etcd.service.consul --CA peerCA
   Created peer/etcd.service.consul.crt from peer/etcd.service.consul.csr signed by peer/peerCA.key
   ```

   The manifest property `properties.etcd.peer_cert` should be set to the certificate in `peer/etcd.service.consul.crt`
   The manifest property `properties.etcd.peer_key` should be set to the certificate in `peer/etcd.service.consul.key`

### Custom SSL Certificate Generation

If you already have a CA, or wish to use your own names for clients and
servers, please note that the common-names "etcdCA" and "clientName" are
placeholders and can be renamed provided that all clients client certificate.
The server certificate must have the common name `etcd.service.consul` and
must specify `etcd.service.consul` and `*.etcd.service.consul` as Subject
Alternative Names (SANs).

## Failure Recovery

### TLS Certificate Issues

A common source of failure is TLS certification configuration.  If you have a failed
deploy and see errors related to certificates, authorities, "crypto", etc. it's a good
idea to confirm that:

* all etcd-related certificates and keys in your manifest are correctly PEM-encoded;
* certificates match their corresponding keys;
* certificates have been signed by the appropriate CA certificate; and
* the YAML syntax of your manifest is correct.

### Failed Deploys, Upgrades, Split-Brain Scenarios, etc.
 
In the event that the etcd cluster ends up in a bad state that is difficult
to debug, you have the option of stopping etcd on each node, removing its
data store, and then restarting the process:

```
monit stop etcd (on all nodes in etcd cluster)
rm -rf /var/vcap/store/etcd/* (on all nodes in etcd cluster)
monit start etcd (one-by-one on each node in etcd cluster)
```

There are often more graceful ways to solve specific issues, but it is hard
to document all of the possible failure modes and recovery steps. As long as
your etcd cluster does not contain critical data that cannot be repopulated,
this option is safe and will probably get you unstuck. If you are debugging
an etcd server cluster in the context of a Cloud Foundry and/or Diego
deployment, it should be safe to follow the above steps.

CAVEAT: There is currently a known issue with the new TCP Routing feature in
Cloud Foundry where the Routing API stores non-ephemeral data pertaining to
Router Groups in etcd.  At the time of this writing, there is work in progress
for the Routing API to store such data outside of etcd.
