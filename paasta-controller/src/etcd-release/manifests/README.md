# Deploy to bosh-lite

With a fresh bosh-lite, you can update the cloud-config and deploy a 3-node cluster without SSL:

```
export BOSH_ENVIRONMENT=<name>
export BOSH_DEPLOYMENT=etcd
bosh2 update-cloud-config manifests/bosh-lite/cloud-config.yml
bosh2 upload-release
bosh2 deploy manifests/bosh-lite/3-node-no-ssl.yml
```

To see that the 3 nodes are running in 3 different AZs (an artificial concept in bosh-lite, but good to see the allocation):

```
$ bosh2 instances
Instance                                   Process State  AZ  IPs
etcd/02b9bbee-d165-4467-b39d-1b2c3a552726  running        z1  10.244.128.2
etcd/35effa1a-834f-42ab-bed2-f01890b05e00  running        z3  10.244.128.4
etcd/54825aef-bd63-4ba2-84ff-3fc0586da45f  running        z2  10.244.128.3
```

## SSL

To deploy or upgrade the cluster to include SSL use `etcd-with-ssl.yml`:

```
bosh2 deploy manifests/bosh-lite/3-node-with-ssl.yml
```

This will also include a `consul` cluster to provide DNS across the cluster, and an SSL cert that supports the shared DNS `etcd.service.cf.internal`.

```
$ bosh2 instances
Instance                                     Process State  AZ  IPs
consul/06a1483f-f0fb-43a8-b657-7fe043a726b3  running        z1  10.244.128.5
consul/ad3e3d93-e6f4-4a80-8343-49b8ec7b7ce0  running        z2  10.244.128.6
consul/ffaed5d1-720c-4100-99c1-ca4d68c95ca9  running        z3  10.244.128.7
etcd/02b9bbee-d165-4467-b39d-1b2c3a552726    running        z1  10.244.128.2
etcd/35effa1a-834f-42ab-bed2-f01890b05e00    running        z3  10.244.128.4
etcd/54825aef-bd63-4ba2-84ff-3fc0586da45f    running        z2  10.244.128.3
```

## 1 node

For your enjoyment, there is an operator patch provided to deploy a single server etcd. Apply `1-node.ops.yml` to either of the `3-node*.yml` base manifests. Either:

```
bosh2 deploy manifests/bosh-lite/3-node-no-ssl.yml -o manifests/bosh-lite/1-node.ops.yml
bosh2 deploy manifests/bosh-lite/3-node-with-ssl.yml -o manifests/bosh-lite/1-node.ops.yml
```


## Run acceptance tests

Acceptance tests for this release are runnable as a separate deployment called `eats`:

```
bosh2 -d eats int manifests/bosh-lite/eats.yml --var-errs
```

You will be prompted for missing variables to describe access to your bosh-lite.

If you're using a vagrant/bosh-lite, then the following should work:

```
bosh2 -d eats deploy manifests/bosh-lite/eats.yml -l manifests/bosh-lite/eats-vars-vagrant-boshlite.yml
```

The variables for your own custom bosh-lite might be found inside the internal `~/.bosh/config` after you logged in or the `creds.yml` from when you created your bosh-lite via `bosh-deployment` repo.

Your deployment now has an `acceptance-tests` errand:

```
bosh2 -d eats run-errand acceptance-tests
```
