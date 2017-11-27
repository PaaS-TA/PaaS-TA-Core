[![slack.cloudfoundry.org](https://slack.cloudfoundry.org/badge.svg)](https://cloudfoundry.slack.com/messages/capi/)

# Cloud Foundry CAPI Bosh Release

This is the [bosh release](http://bosh.io/docs/release.html) for Cloud Foundry's [Cloud Controller API](https://github.com/cloudfoundry/cloud_controller_ng). 

**CI**: [CAPI Concourse Pipelines](https://capi.ci.cf-app.com)

## Components

* [Cloud Controller](https://github.com/cloudfoundry/cloud_controller_ng): The primary API of Cloud Foundry.
* [Cloud Controller Clock](https://github.com/cloudfoundry/cloud_controller_ng): Triggers periodic jobs for the Cloud Controller.
* [Cloud Controller Workers](https://github.com/cloudfoundry/cloud_controller_ng): Execute background jobs for the Cloud Controller.
* [Webdav Blobstore](https://github.com/cloudfoundry/capi-release/tree/develop/jobs/blobstore): An optional stand-alone blobstore for the Cloud Controller. 
* [NFS Mounter](https://github.com/cloudfoundry/capi-release/tree/develop/jobs/nfs_mounter): Connects Cloud Controller with an NFS blobstore.
* [CC Uploader](https://github.com/cloudfoundry/cc-uploader): Uploads files from [Diego](https://github.com/cloudfoundry/diego-release) to the Cloud Controller.
* [TPS Watcher](https://github.com/cloudfoundry/tps): Reports crash events from Diego to the Cloud Controller.

For more details on the integration between Diego and Capi Release, see [Diego Design Notes](https://github.com/cloudfoundry/diego-design-notes).

#### The following components have been replaced by direct communication between Cloud Controller and the Diego BBS API:
* [Stager](https://github.com/cloudfoundry/stager): Proxies staging requests from Cloud Controller to the Diego BBS API.
* [Nsync](https://github.com/cloudfoundry/nsync): Proxies task and app start requests from Cloud Controller to the Diego BBS API. Synchronizes health and state of apps and tasks between the BBS and Cloud Controller.
* [TPS Listener](https://github.com/cloudfoundry/tps): Proxies metrics information from Diego BBS API to Cloud Controller.

#### Deprecated:

* [NFS Debian Server](https://github.com/cloudfoundry/capi-release/tree/develop/jobs/debian_nfs_server): An optional stand-alone blobstore for the Cloud Controller. Replaced by Webdav Blobstore.

## Configuring Release

* [Deploying Cloud Foundry](https://docs.cloudfoundry.org/deploying/index.html)
* [Blobstore Configuration](https://docs.cloudfoundry.org/deploying/common/cc-blobstore-config.html)
* [TLS Configuration](https://github.com/cloudfoundry/capi-release/blob/develop/docs/tls-configuration.md)

## Contributing

* Read [Contribution Guidelines](https://github.com/cloudfoundry/capi-release/blob/develop/CONTRIBUTING.md)
* Public [Pivotal Tracker](https://www.pivotaltracker.com/n/projects/966314) project showing current team priorities
