# go-loggregator [![ci][ci-badge]][ci-pipeline][![slack.cloudfoundry.org][slack-badge]][loggregator-slack][![GoDoc][go-doc-badge]][go-doc]

This is a golang client library for [Loggregator][loggregator].

## Versions

At present, Loggregator supports two API versions: v1 (UDP) and v2 (gRPC).
This library provides clients for both versions.

Note that this library is also versioned. Its versions have *no* relation to
the Loggregator API. Presently, v2.0.0 is the most recent release.

## Usage

This repository should be imported as:

`import loggregator "code.cloudfoundry.org/go-loggregator"`

## Example

Example implementation of the client is provided in `examples/main.go`.

Build the example client by running `go build -o client main.go`

Collocate the `client` with a metron agent and set the following environment
variables: `CA_CERT_PATH`, `CERT_PATH`, `KEY_PATH`

[ci-badge]:                 https://loggregator.ci.cf-app.com/api/v1/teams/main/pipelines/go-loggregator/jobs/tests/badge
[ci-pipeline]:              https://loggregator.ci.cf-app.com/teams/main/pipelines/go-loggregator
[slack-badge]:              https://slack.cloudfoundry.org/badge.svg
[loggregator-slack]:        https://cloudfoundry.slack.com/archives/loggregator
[loggregator]:              https://github.com/cloudfoundry/loggregator
[go-doc-badge]:             https://godoc.org/code.cloudfoundry.org/go-loggregator?status.svg
[go-doc]:                   https://godoc.org/code.cloudfoundry.org/go-loggregator
