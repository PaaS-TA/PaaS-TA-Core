## BBS Benchmark

**Note**: This repo is meant to be used inside a GOPATH that points to a locally cloned [diego-release](https://github.com/cloudfoundry/diego-release). Alternatively you can use the [generate-benchmarks-manifest script](https://github.com/cloudfoundry/diego-release/blob/develop/scripts/generate-benchmarks-manifest) from `diego-release` to generate a benchmark deployment manifest. Then, follow these instructions to deploy the benchmark errand and run it:

```
bosh deployment /path/to/diego-benchmarks.yml
bosh deploy
bosh run errand benchmark-bbs
```

This test suite simulates the load of a CF + Diego deployment against a Diego BBS API server.

### Running the Tests

The following instructions demonstrate how to run the BBS benchmarks against
a CF + Diego deployment on [BOSH-Lite](https://github.com/cloudfoundry/bosh-lite).

#### Disable Convergence and Enable Locket

To prevent it from interfering with the test data, the BBS should run with
convergence disabled. This can be done by setting the
`properties.diego.bbs.convergence.repeat_interval_in_seconds` property in the
Diego deployment manifest to a sufficiently large value (for example, `1000000`)
so that convergence will not execute during the test run.

The BBS Benchmark test suite now also maintains cell registrations with the Locket
service, so Locket must be deployed before running it. This can be done via the
`-Q` option on the Diego manifest-generation script.


#### Stop the Brain and Bridge VMs

Before running these tests, the Diego Brain, CC-Bridge, and Cell VMs should be
stopped to prevent them from interfering with the test data. On a BOSH-Lite
deployment, this can be done by running the following BOSH commands:

```
bosh stop brain_z1 0
bosh stop cc_bridge_z1 0
bosh stop cell_z1 0
```


#### Create Configuration JSON

In order to run the benchmark test suite, you will need to create the necessary configuration file.

Example BOSH-Lite configuration file for a MySQL Backend:

```bash
cat > config.json <<EOF
{
  "desired_lrps": 5000,
  "num_trials": 10,
  "num_reps": 5,
  "num_populate_workers": 10,
  "bbs_address": "https://10.244.16.2:8889",
  "bbs_client_http_timeout": "10s",
  "bbs_client_cert": "$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.crt",
  "bbs_client_key": "$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.key",
  "locket_address": "10.244.16.2:8891",
  "locket_client_cert_file": "$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.crt",
  "locket_client_key_file": "$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.key",
  "skip_cert_verify": true,
  "encryption_keys": {
    "key1": "a secure passphrase"
  },
  "active_key_label": "key1",
  "log_level": "info",
  "log_filename": "test-output.log",
  "database_driver": "mysql",
  "database_connection_string": "diego:diego@tcp(10.244.7.2:3306)/diego"
}
EOF
export CONFIG=$PWD/config.json
```

Example BOSH-Lite configuration file for a Postgres Backend:

```bash
cat > config.json <<EOF
{
  "desired_lrps": 5000,
  "num_trials": 10,
  "num_reps": 5,
  "num_populate_workers": 10,
  "bbs_address": "https://10.244.16.2:8889",
  "bbs_client_http_timeout": "10s",
  "bbs_client_cert": "$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.crt",
  "bbs_client_key": "$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.key",
  "locket_address": "10.244.16.2:8891",
  "locket_client_cert_file": "$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.crt",
  "locket_client_key_file": "$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.key",
  "skip_cert_verify": true,
  "encryption_keys": {
    "key1": "a secure passphrase"
  },
  "active_key_label": "key1",
  "log_level": "info",
  "log_filename": "test-output.log",
  "database_driver": "postgres",
  "database_connection_string": "postgres://diego:admin@10.244.0.30:5524/diego"
}
EOF
export CONFIG=$PWD/config.json
```

#### Run the Tests

Run `ginkgo` with the following flags:

```
ginkgo -- -config=$CONFIG
```

### Error Tolerance

To change the fractional error tolerance allowed, add the following property to the configuration JSON:
```
  "error_tolerance": 0.025,
```

### Percent Writes

To change the write load on the database, add the following property to the configuration JSON:

```
  "percent_writes": 5.0,
```

This property specifies the percentage of the total LRPs desired that the benchmarks will attempt to
write on each trial.

### Local Route Emitters

To simulate the behavior of having local route emitters on each cell, the following property can be specified in the JSON configuration:

```
  "local_route_emitters": true,
```

### Metrics

To emit metrics to Datadog, add the following properties to the configuration JSON:

```
  "datadog_api_key": "$DATADOG_API_KEY",
  "datadog_app_key": "$DATADOG_APP_KEY",
  "metric_prefix": "$METRIC_PREFIX",
```

To save the benchmark metrics to an S3 bucket, add the following properties:

```
 "aws_access_key_id": "$AWS_ACCESS_KEY_ID",
 "aws_secret_access_key": "$AWS_SECRET_ACCESS_KEY",
 "aws_bucket_name": "$AWS_BUCKET_NAME",
 "aws_region": "$AWS_REGION",
```

#### Collected metrics

* **ConvergenceGathering**: The time to complete a convergence loop.
* **FetchActualLRPsAndSchedulingInfos**: The time to fetch information about
all `ActualLRPs` and `DesiredLRPs` known to the BBS.
* **NsyncBulkerFetching**: The time to fetch information about new
`DesiredLRPs` from the `nsync-bulker` process.
* **RepBulkFetching**: The time to fetch a cell's expected `ActualLRPs` from the BBS.
* **RepBulkLoop** The time to calculate `ActualLRP` statistics and enqueue
operations based on the results.
* **RepClaimActualLRP**: The time required to claim an `ActualLRP` within the BBS.
* **RepStartActualLRP**: The time required to register an `ActualLRP` with the BBS as "started".


Example:
```
{
  "Timestamp" : 1466806960,
  "Measurement" : {
    "Name" : "BBS' internal gathering of LRPs",
    "Info" : {
      "MetricName" : "ConvergenceGathering"
    },
    "Results" : [
      0.048770786
    ]
    "Average" : 0.048770786,
    "Smallest" : 0.048770786,
    "Largest" : 0.048770786,
    "AverageLabel" : "Average Time",
    "SmallestLabel" : "Fastest Time",
    "LargestLabel" : "Slowest Time",
    "Order" : 5,
    "Units" : "s",
    "StdDeviation" : 0,
  }
}
```

Measurement fields:

* **Name**: The metric name.
* **Info**: Additional reporter info for this metric.
* **Results**: The metric results.
* **Average, Smallest, Largest**: The average, smallest, and largest values in Results.
* **AverageLabel, SmallestLabel, LargestLabel**: Labels for the average, smallest, and largest values.
* **Order**: The index of this metric out of all metrics in this run.
* **Units**: The units of measurement for this metric.
* **StdDeviation**: The standard deviation of the results.

