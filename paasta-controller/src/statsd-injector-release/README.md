# statsd-injector [![slack.cloudfoundry.org][slack-badge]][loggregator-slack]
Companion component to Metron that receives Statsd and emits to Metron.

## Usage

`statsd_injector` is to be colocated with a `metron_agent`. It receives
metrics via UDP and will emit to `metron-agent` via the [Loggregator v2
api][loggregator-api] which is built on
[gRPC][grpc].

### Creating a release

```
git submodule update --init --recursive
bosh create-release
```

### Deployment

These instructions are colocating the job with `metron_agent`.

1. Generate certs for `statsd_injector` by running
   `scripts/generate-certs <loggregator-ca.crt> <loggregator-ca.key>`. The
   Loggregator CA cert is generated from the [loggregator
   release](https://github.com/cloudfoundry/loggregator/blob/develop/docs/cert-config.md).

   This script generates two files in `./statsd-injector-certs` that are used
   as bosh properties.

1. Add the release to your deployment manifest.

   ```diff
   releases:
   +  - name: statsd-injector
   +    version: latest
      - name: loggregator
        version: latest
   ```

   Then `bosh upload release` the latest [`statsd-injector-release` bosh release][bosh-release].

1. Colocate the job that has `metron_agent`.

    ```diff
    jobs:
      - name: some_job_z1
      templates:
      - name: metron_agent
        release: loggegator
    + - name: statsd_injector
    +   release: statsd-injector
      instances: 1
      resource_pool: default
      networks:
        - name: default
      properties:
        metron_agent:
          zone: z1
    +   statsd_injector:
    +     deployment: some_deployment_name
        loggregator:
          tls:
            ca_cert: loggregator_cert
            metron:
              cert: metron_cert
              key: metron_key
    +       statsd_injector:
    +         cert: <cert from script generation>
    +         key: <key from script generation>
    ```

   Then `bosh deploy` this updated manifest.

1. Send it a metric

   You can emit statsd metrics to the injector by sending a correctly formatted
   message to udp port 8125 on the job's VM.

   As an example using `nc`:

   ```bash
   echo "origin.some.counter:1|c" | nc -u -w0 127.0.0.1 8125
   ```

   *NOTE:* The injector expects the the name of the metric to be of the form `<origin>.<metric_name>`

1. Validate the metric can be seen.

   Assuming you are using `statsd-injector` with CF Release, you can use the
   [CF Nozzle plugin][cf-nozzle-plugin]

   ```bash
   cf nozzle -filter CounterEvent | grep <metric_name>
   ```


[slack-badge]:          https://slack.cloudfoundry.org/badge.svg
[loggregator-slack]:    https://cloudfoundry.slack.com/archives/loggregator
[loggregator-api]:      https://github.com/cloudfoundry/loggregator-api
[grpc]:                 https://github.com/grpc/
[bosh-release]:         http://bosh.io/releases/github.com/cloudfoundry/statsd-injector-release?all=1
[cf-nozzle-plugin]:     https://github.com/cloudfoundry-community/firehose-plugin     
