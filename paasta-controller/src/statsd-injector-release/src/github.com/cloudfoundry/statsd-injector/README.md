# statsd-injector
Companion component to Metron that receives Statsd and emits Dropsonde to Metron

## Getting started

The following instructions may help you get started with statsd-injector in a
standalone environment.

### External Dependencies

To start, you must have the latest version of Go installed and on your PATH.

Then, choose one of the following:

#### Running from the bosh release

The [statsd-injector-release repository](https://github.com/cloudfoundry/statsd-injector-release) contains a
complete GOPATH with all of the `statsd-injector`'s dependencies frozen at known working versions.  Working
from that repository will guarantee the best compatibility with the bosh release.

To work from within the bosh release:

1. Clone https://github.com/cloudfoundry/statsd-injector-release
1. From the statsd-injector-release directory:
  1. Run `git submodule update --init --recursive`
  1. If you're not using `direnv`, run `source .envrc`

#### Running with `go get`

- Set your GOPATH, as described in http://golang.org/doc/code.html
- Run `go get github.com/cloudfoundry/statsd-injector`

## Running Tests

We are using [Ginkgo](https://github.com/onsi/ginkgo), to run tests. To run the tests, execute the following
from the top level directory of this repository:

```bash
ginkgo -r -race -randomizeAllSpecs
```

## Including statsd-injector in a bosh deployment
As an example, if you want the injector to be present on loggregator boxes, add the following in `cf-lamb.yml`

```diff
   loggregator_templates:
   - name: doppler
     release: (( lamb_meta.release.name ))
   - name: syslog_drain_binder
     release: (( lamb_meta.release.name ))
   - name: metron_agent
     release: (( lamb_meta.release.name ))
+  - name: statsd-injector
+    release: (( lamb_meta.release.name ))
```

## Emitting metrics to the statsd-injector
You can emit statsd metrics to the injector by sending a correctly formatted message to udp port 8125

As an example using netcat:

```
echo "origin.some.counter:1|c" | nc -u -w0 127.0.0.1 8125
```

You should see the metric come out of the firehose.

The injector expects the the name of the metric to be of the form `<origin>.<metric_name>`
