# Routing Acceptance Tests

This test suite exercises [Cloud Foundry Routing](https://github.com/cloudfoundry-incubator/routing-release) deployment.

**Note**: This repository should be imported as `code.cloudfoundry.org/routing-acceptance-tests`.

## Running Acceptance tests

### Test setup

To run the Routing Acceptance tests, you will need:
- a running deployment of [routing-release](https://github.com/cloudfoundry-incubator/routing-release)
- the latest version of the [rtr CLI](https://github.com/cloudfoundry-incubator/routing-api-cli/releases)
- an environment variable `CONFIG` which points to a `.json` file that contains the router api endpoint
- environment variable GOPATH set to root directory of [routing-release](https://github.com/cloudfoundry-incubator/routing-release)
```bash
git clone https://github.com/cloudfoundry-incubator/routing-release.git
cd routing-release
./scripts/update
source .envrc
```

The following commands will create a config file `integration_config.json` for a [bosh-lite](https://github.com/cloudfoundry/bosh-lite) installation and set the `CONFIG` environment variable to the path for this file. Edit `integration_config.json` as appropriate for your environment.


```bash
cd ~/workspace/routing-release/src/code.cloudfoundry.org/routing-acceptance-tests/
cat > integration_config.json <<EOF
{
  "addresses": ["10.244.14.2"],
  "api": "api.bosh-lite.com",
  "admin_user": "admin",
  "admin_password": "admin",
  "skip_ssl_validation": true,
  "use_http":true,
  "apps_domain": "bosh-lite.com",
  "include_http_routes": true,
  "oauth": {
    "token_endpoint": "https://uaa.bosh-lite.com",
    "client_name": "tcp_emitter",
    "client_secret": "tcp-emitter-secret",
    "port": 443,
    "skip_ssl_validation": true
  }
}
EOF
export CONFIG=$PWD/integration_config.json
```

Note:
- The `addresses` property contains the IP addresses of the TCP Routers and/or the Load Balancer's IP address. IP `10.24.14.2` is IP address of `tcp_router_z1/0` job in routing-release. If this IP address happens to be different in your deployment then change the entry accordingly.
- `admin_user` and `admin_password` properties refer to the admin user used to perform a CF login with the cf CLI.
- `skip_ssl_validation` is used for the cf CLI when targeting an environment.
- `include_http_routes` boolean used to run tests for the experimental HTTP routing endpoints of the Routing API.

### Running the tests

After correctly setting the `CONFIG` environment variable, the following command will run the tests:

```
    ./bin/test
```
