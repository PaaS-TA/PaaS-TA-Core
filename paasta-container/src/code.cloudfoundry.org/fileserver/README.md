File Server
===========

**Note**: This repository should be imported as `code.cloudfoundry.org/fileserver`.

Bob Loblaw's Blob Store

## Uploading Droplets & Build Artifacts

Uploading droplets & build artifacts via CC involves crafting a correctly-formed multipart request. For Droplets we also poll until the async job completes.

To specify a remote cloud controller to test against, use the following environment variables:

CC_ADDRESS the hostname for a deployed CC
CC_USERNAME, CC_PASSWORD the basic auth credentials for the droplet upload endpoint
CC_APPGUID a valid app guid on that deployed CC

####Learn more about Diego and its components at [diego-design-notes](https://github.com/cloudfoundry/diego-design-notes)
