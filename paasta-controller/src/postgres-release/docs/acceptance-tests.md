# Postgres-release Acceptance Tests (PGATS)

The acceptance tests run several deployments of the postgres-release in order to exercise a variety of scenario:
- Verify that customizable configurations are properly reflected in the PostgreSQL server
  - Roles
  - Databases
  - Database extensions
  - Properties (e.g. max_connections)
- Test supported upgrade paths from previous versions

You can run PGATS in two ways:

* [locally](#running-pgats-locally)
* [as BOSH errand](#running-pgats-as-bosh-errand)

## Running PGATS locally

### Environment setup

* Upload to the BOSH director the latest stemcell and your dev postgres-release:

```
bosh upload-stemcell STEMCELL_URL_OR_PATH_TO_DOWNLOADED_STEMCELL
bosh create-release --force
bosh upload-release
```

* The acceptance tests are written in Go. Make sure that:
 - Golang (>=1.7) is installed on the machine
 - the postgres-release is inside your $GOPATH

* PGATS must have access to the target BOSH director and to the postgres VM deployed from it.
If you are testing using a bosh-lite, make sure you’ve run `bin/add-route` to setup routing rules.

* Tests make use of BOSH v2 manifests.
Make sure that the BOSH director is configured with the [cloud_config.yml](https://bosh.io/docs/cloud-config.html#update).

* PGATS use bosh-cli director package for programmatic access to the Director API.
It requires the Director to be configured with verifiable [certificates](https://bosh.io/docs/director-certs.html).


### Configuration

An example config yml for bosh-lite would look like:

```
cat > pgats_config.yml << EOF
---
bosh:
  target: 192.168.50.4
  username: admin
  password: admin
  director_ca_cert: |+
    -----BEGIN CERTIFICATE-----
    MIIDtzCCAp+gAwIBAgIJAMZ/qRdRamluMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
    BAYTAkFVMRMwEQYDVQQIEwpTb21lLVN0YXRlMSEwHwYDVQQKExhJbnRlcm5ldCBX
    aWRnaXRzIFB0eSBMdGQwIBcNMTYwODI2MjIzMzE5WhgPMjI5MDA2MTAyMjMzMTla
    MEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIEwpTb21lLVN0YXRlMSEwHwYDVQQKExhJ
    bnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAw
    ggEKAoIBAQDN/bv70wDn6APMqiJZV7ESZhUyGu8OzuaeEfb+64SNvQIIME0s9+i7
    D9gKAZjtoC2Tr9bJBqsKdVhREd/X6ePTaopxL8shC9GxXmTqJ1+vKT6UxN4kHr3U
    +Y+LK2SGYUAvE44nv7sBbiLxDl580P00ouYTf6RJgW6gOuKpIGcvsTGA4+u0UTc+
    y4pj6sT0+e3xj//Y4wbLdeJ6cfcNTU63jiHpKc9Rgo4Tcy97WeEryXWz93rtRh8d
    pvQKHVDU/26EkNsPSsn9AHNgaa+iOA2glZ2EzZ8xoaMPrHgQhcxoi8maFzfM2dX2
    XB1BOswa/46yqfzc4xAwaW0MLZLg3NffAgMBAAGjgacwgaQwHQYDVR0OBBYEFNRJ
    PYFebixALIR2Ee+yFoSqurxqMHUGA1UdIwRuMGyAFNRJPYFebixALIR2Ee+yFoSq
    urxqoUmkRzBFMQswCQYDVQQGEwJBVTETMBEGA1UECBMKU29tZS1TdGF0ZTEhMB8G
    A1UEChMYSW50ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkggkAxn+pF1FqaW4wDAYDVR0T
    BAUwAwEB/zANBgkqhkiG9w0BAQUFAAOCAQEAoPTwU2rm0ca5b8xMni3vpjYmB9NW
    oSpGcWENbvu/p7NpiPAe143c5EPCuEHue/AbHWWxBzNAZvhVZBeFirYNB3HYnCla
    jP4WI3o2Q0MpGy3kMYigEYG76WeZAM5ovl0qDP6fKuikZofeiygb8lPs7Hv4/88x
    pSsZYBm7UPTS3Pl044oZfRJdqTpyHVPDqwiYD5KQcI0yHUE9v5KC0CnqOrU/83PE
    b0lpHA8bE9gQTQjmIa8MIpaP3UNTxvmKfEQnk5UAZ5xY2at5mmyj3t8woGdzoL98
    yDd2GtrGsguQXM2op+4LqEdHef57g7vwolZejJqN776Xu/lZtCTp01+HTA==
    -----END CERTIFICATE-----
cloud_configs:
  default_azs: [z1]
  default_networks:
  - name: default
  default_persistent_disk_type: 10GB
  default_vm_type: m3.medium
EOF
export PGATS_CONFIG=$PWD/pgats_config.yml
```

The full set of config parameters is explained below.

`bosh`parameters are used to connect to the BOSH director that would host the test environment:

* `bosh.target` (required) Public BOSH director ip address
* `bosh.username` (required) Username for the BOSH director login
* `bosh.password` (required) Password for the BOSH director login
* `bosh.director_ca_cert` (required) BOSH director CA Cert

`cloud_config` parameters are used to generate a BOSH v2 manifest that matches your IaaS configuration:

* `cloud_config.default_azs` List of vailability zones. It defaults to `[z1]`.
* `cloud_config.default_networks` List of networks. It defaults to `[{name: private}]`.
* `cloud_config.default_persistent_disk_type` Persistent disk type. It defaults to `10GB`.
* `cloud_config.default_vm_type` VM type. It defaults to `m3.medium`.

Other paramaters:

* `postgres_release_version` The postgres-release version to test. If not specified, the latest uploaded to the director is used.
* `postgresql_version` The PostgreSQL version that is expected to be deployed. You only need to specify it if your changes include a PostgreSQL version upgrade.
If not specified, we expect that the one in the latest published postgres-release is deployed.


### Running

Run all the tests with:

```
PGATS_CONFIG=[pgats_config.yml] <postgres-release>/src/acceptance-tests/scripts/test
```

Run a specific set of tests with:

```
PGATS_CONFIG=[pgats_config.yml] <postgres-release>/src/acceptance-tests/scripts/test <some test packages>
```

The `PGATS_CONFIG` environment variable must point to the absolute path of the [configuration file](#configuration)


## Running PGATS as BOSH errand

### Environment setup
* Install the BOSH command line Interface (CLI) v2+.
   Please refer to [BOSH CLI documentation](https://bosh.io/docs/cli-v2.html#install).


* Upload to the BOSH director the latest stemcell and your dev postgres-release:

```
bosh upload-stemcell STEMCELL_URL_OR_PATH_TO_DOWNLOADED_STEMCELL
bosh create-release --force
bosh upload-release
```

* Tests make use of BOSH v2 manifests. 
Make sure that the BOSH director is configured with the [cloud_config.yml](https://bosh.io/docs/cloud-config.html#update).

* PGATS use bosh-cli director package for programmatic access to the Director API. 
It requires the Director to be configured with verifiable [certificates](https://bosh.io/docs/director-certs.html).

### Running
Generate the manifest.

- You must provide in input a variable file containing BOSH director url, user, password, and ca certificate. See by way of [example](blob/master/templates/v2/bosh-lite/pgats-vars.yml) a variable file for bosh-lite.

- You can provide in input an [operation file](https://bosh.io/docs/cli-ops-files.html). You can use it:
  - to customize the properties (see [example](blob/master/templates/v2/operations/pgats-props.yml))
  - to override the configuration if your BOSH director [cloud-config](http://bosh.io/docs/cloud-config.html) is not compatible

```
~/workspace/postgres-release/scripts/generate-pgats-manifest-v2 \
   -v VARIABLE-FILE-PATH \
   -o OPERATION-FILE-PATH > pgats_errand.yml

```

Deploy and run the errand:

```
bosh pgats-errand deploy pgats_errand.yml
bosh run-errand acceptance-tests
```
