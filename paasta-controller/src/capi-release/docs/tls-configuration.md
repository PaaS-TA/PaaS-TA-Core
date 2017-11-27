Components communicating with CC via its internal API (for example: Loggregator, BBS, and TPS) will do so over mutual TLS.
This is part of an effort to have all Cloud Foundry internal traffic be done over mutual TLS in lieu of basic auth.
The CC and other components must now be configured with several new certificates to establish these mTLS connections.
For most deployments, use a shared CA between CF and Diego deployments.

# For new deployments

## Generating the shared CA certificate and CC Server certificate

Please run `cf-release/scripts/generate-cf-diego-certs`. This script will create a directory called cf-diego-certs.
Within this directory will be a CA, to be shared between your cf-release and diego-release deployments.

Contents of file                                 | Deployment | Property
------------------------------------------------ | ---------- | ---------
`cf-release/cf-diego-certs/cf-diego-ca.crt`      | CF         | `properties.cc.mutual_tls.ca_cert`
`cf-release/cf-diego-certs/cf-diego-ca.crt`      | Diego      | `properties.capi.tps.cc.ca_cert`
`cf-release/cf-diego-certs/cf-diego-ca.crt`      | Diego      | `properties.capi.cc_uploader.cc.ca_cert`
`cf-release/cf-diego-certs/cf-diego-ca.crt`      | Diego      | `properties.capi.cc_uploader.mutual_tls.ca_cert`
`cf-release/cf-diego-certs/cf-diego-ca.crt`      | Diego      | `properties.tls.ca_cert`
`cf-release/cf-diego-certs/cloud-controller.crt` | CF         | `properties.cc.mutual_tls.public_cert`
`cf-release/cf-diego-certs/cloud-controller.key` | CF         | `properties.cc.mutual_tls.private_key`

## Generating Diego client certificates

Please run `diego-release/scripts/generate-diego-certs`.  For example, if you ran `cf-release/scripts/generate-cf-diego-certs`
as per the step above, you would now run `scripts/generate-diego-certs cf-diego-ca /path/to/cf-release/cf-diego-certs`.

Contents of file                                         | Deployment | Property
-------------------------------------------------------- | ---------- | ---------
`diego-release/diego-certs/tps-certs/client.crt`         | Diego      | `properties.capi.tps.cc.client_cert`
`diego-release/diego-certs/tps-certs/client.key`         | Diego      | `properties.capi.tps.cc.client_key`
`diego-release/diego-certs/cc-uploader-certs/client.key` | Diego      | `properties.capi.cc_uploader.cc.client_key`
`diego-release/diego-certs/cc-uploader-certs/client.key` | Diego      | `properties.capi.cc_uploader.cc.client_key`


# For an existing deployment

## Shared CA certificate

We will use the CA cert configured for Diego's deployment to populate
`properties.cc.mutual_tls.ca_cert`, `properties.capi.tps.cc.ca_cert`, and `properties.capi.cc_uploader.cc.ca_cert`.

## Generating the Cloud Controller Server certificate

Given an existing CA, with the .crt and .key files found in `/path/to/CA`, we can generate a signing request and sign it with that CA

```
$ certstrap --depot-path /path/to/CA request-cert --passphrase '' --common-name cloud-controller-ng.service.cf.internal
$ certstrap --depot-path /path/to/CA sign cloud-controller-ng.service.cf.internal --CA <CA NAME>
```

Contents of file                                          | Deployment | Property
--------------------------------------------------------- | ---------  | --------
`/path/to/CA/cloud-controller-ng.service.cf.internal.crt` | CF         | `properties.cc.mutual_tls.public_cert`
`/path/to/CA/cloud-controller-ng.service.cf.internal.key` | CF         | `properties.cc.mutual_tls.private_key`

## Generating the TPS client certificate

Please run `diego-release/scripts/generate-tps-certs`, this will guide you on how to generate the values below.
Use the same CA as for the steps above.

Contents of file                                 | Deployment | Property
------------------------------------------------ | ---------- | --------
`diego-release/diego-certs/tps-certs/client.crt` | Diego      | `properties.capi.tps.cc.client_cert`
`diego-release/diego-certs/tps-certs/client.key` | Diego      | `properties.capi.tps.cc.client_key`.


## Generating the CC-Uploader certificates

Please run `diego-release/scripts/generate-cc-uploader-certs` and `diego-release/scripts/generate-rep-certs`, this will guide you on how to generate the values below.
Use the same CA as for the steps above.

This script will generate two sets of certificates:

1. Generates client certificates for enabling mTLS communication from the CC Uploader to CC
1. Enabling mTLS communication from Diego to CC Uploader


Contents of file                                            | Deployment | Property
----------------------------------------------------------- | ---------- | --------
`diego-release/diego-certs/cc-uploader-certs/cc/client.crt` | Diego      | `properties.capi.cc_uploader.cc.client_cert`
`diego-release/diego-certs/cc-uploader-certs/cc/client.key` | Diego      | `properties.capi.cc_uploader.cc.client_key`
`diego-release/diego-certs/cc-uploader-certs/server.crt`    | Diego      | `properties.capi.cc_uploader.mutual_tls.server_cert`
`diego-release/diego-certs/cc-uploader-certs/server.key`    | Diego      | `properties.capi.cc_uploader.mutual_tls.server_key`
`diego-release/diego-certs/rep-certs/client.crt`            | Diego      | `properties.tls.cert`
`diego-release/diego-certs/rep-certs/client.key`            | Diego      | `properties.tls.key`

If you run into trouble, please feel free to reach out to us on [slack](https://cloudfoundry.slack.com/messages/capi/).
