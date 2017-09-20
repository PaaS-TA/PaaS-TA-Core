# check-a-record
---

This is a command line utility for checking if a host resolves to one or more DNS A Record(s).
If at least one A record exists for a given host, check-a-record will print the IPs that the
host resolves to and exit with return code 0. Otherwise, check-a-record prints an error
message and exits with return code 1.

* [CI](https://mega.ci.cf-app.com/pipelines/check-a-record)

## Dependencies

The following should be installed on your local machine
- Golang (>= 1.5)

If using homebrew, these can be installed with:

```
brew install go
```

## Installation

```
go get github.com/cloudfoundry-incubator/check-a-record
```

## Usage

The `check-a-record` command can be invoked on the command line and will display it's usage.

```
usage: check-a-record <host>
```

## Running Tests

The acceptance tests run inside of a docker container to allow a DNS server to be run on port
53. To run the tests:

```
./scripts/test
```
