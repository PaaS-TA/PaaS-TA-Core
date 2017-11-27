# Cloud Foundry Identity Tools

A collection of useful standalone projects that are used as part of
the Identity Management solution of Cloud Foundry.

## Statsd

A Spring Boot application that can be deployed as a war into any servlet container and
collects and emit UAA metrics to udp port 8125.

## Batch

Some batch jobs used to keep the UAA and Cloudcontroller databases
synchronized.  Deployed as a servlet application, e.g. if deployed as
`/batch` then there is a Spring Batch Admin UI and set of JSON
endpoints at that URL.

##  Szxcvbn

Scala port of the zxcvbn project (MIT licensed). See the subproject
readme file for more details.

## License

All projects individually licensed (ASL 2.0 unless otherwise stated).
