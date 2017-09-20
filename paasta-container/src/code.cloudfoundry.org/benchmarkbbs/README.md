## BBS Benchmark

**Note**: This repository should be imported as `code.cloudfoundry.org/benchmarkbbs`.

This test suite simulates the load of a CF + Diego deployment against a Diego BBS API server.


### Running the Tests

The following instructions demonstrate how to run the BBS benchmarks against
a CF + Diego deployment on [BOSH-Lite](https://github.com/cloudfoundry/bosh-lite).

Before you run these tests, stop the Diego Brain and CC-Bridge VMs:

```
bosh stop brain_z1 0
bosh stop cc_bridge_z1 0
```

Run `ginkgo` with the following flags:

```
ginkgo -- \
  -desiredLRPs=5000 \
  -numTrials=10 \
  -numReps=5 \
  -numPopulateWorkers=10 \
  -bbsAddress=https://10.244.16.2:8889 \
  -bbsClientHTTPTimeout=10s \
  -etcdCluster=https://10.244.16.2:4001 \
  -etcdCertFile=$GOPATH/manifest-generation/bosh-lite-stubs/etcd-certs/client.crt \
  -etcdKeyFile=$GOPATH/manifest-generation/bosh-lite-stubs/etcd-certs/client.key \
  -bbsClientCert=$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.crt \
  -bbsClientKey=$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.key \
  -encryptionKey="key1:a secure passphrase" \
  -activeKeyLabel=key1 \
  -logFilename=test-output.log \
  -logLevel=info
```

### Error Tolerance

To change the fractional error tolerance allowed, add the following flag:
```
-errorTolerance=0.025
```

### MySQL Backend

To test with the experimental MySQL backend, add the `-databaseConnectionString`
flag instead of the flags that start with `etcd`. For example:

```
ginkgo -- \
  -desiredLRPs=5000 \
  -numTrials=10 \
  -numReps=5 \
  -numPopulateWorkers=10 \
  -bbsAddress=https://10.244.16.2:8889 \
  -bbsClientHTTPTimeout=10s \
  -databaseConnectionString="diego:diego@tcp(10.244.7.2:3306)/diego" \
  -databaseDriver="mysql" \
  -bbsClientCert=$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.crt \
  -bbsClientKey=$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.key \
  -encryptionKey="key1:a secure passphrase" \
  -activeKeyLabel=key1 \
  -logFilename=test-output.log \
  -logLevel=info
```

### Postgres Backend

To test with the experimental postgres backend, add the `-databaseConnectionString`
flag instead of the flags that start with `etcd`. For example:

```
ginkgo -- \
  -desiredLRPs=5000 \
  -numTrials=10 \
  -numReps=5 \
  -numPopulateWorkers=10 \
  -bbsAddress=https://10.244.16.2:8889 \
  -bbsClientHTTPTimeout=10s \
  -databaseConnectionString="postgres://diego:admin@10.244.0.30:5524/diego" \
  -databaseDriver="postgres" \
  -bbsClientCert=$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.crt \
  -bbsClientKey=$GOPATH/manifest-generation/bosh-lite-stubs/bbs-certs/client.key \
  -encryptionKey="key1:a secure passphrase" \
  -activeKeyLabel=key1 \
  -logFilename=test-output.log \
  -logLevel=info
```

### Metrics

To emit metrics to Datadog, add the following flags:

```
-dataDogAPIKey=$DATADOG_API_KEY \
-dataDogAppKey=$DATADOG_APP_KEY \
-metricPrefix=$METRIC_PREFIX
```

To save the benchmark metrics to an S3 bucket, add the following flags:

```
-awsAccessKeyID=$AWS_ACCESS_KEY_ID \
-awsSecretAccessKey=$AWS_SECRET_ACCESS_KEY \
-awsBucketName=$AWS_BUCKET_NAME \
-awsRegion=$AWS_REGION # defaults to us-west-1
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

