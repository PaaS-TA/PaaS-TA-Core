# loggregator-api
**WARNING**: This repo is a work in progress and will change.

### Table of Contents

* [v2 Envelope](#v2-envelope)
* [v2 Envelope Types](#v2-envelope-types)
  * [Log](#log)
  * [Counter](#counter)
  * [Gauge](#gauge)
  * [Timer](#timer)
* [v2 -> v1 mapping](#v2---v1-mapping)
  * [Envelope](#envelope)
  * [HttpStartStop](#httpstartstop)
  * [LogMessage](#logmessage)
  * [CounterEvent](#counterevent)
  * [ValueMetric](#valuemetric)
  * [ContainerMetric](#containermetric)

## v2 Envelope

| Field     | Description                                                                                                                        |
|-----------|------------------------------------------------------------------------------------------------------------------------------------|
| timestamp | UNIX timestamp in nanoseconds.                                                                                                     |
| source_id | Deterministic id for source of envelope. (e.g. `984992f6-3cfb-4417-9321-786ee5233e9c` for an app or `doppler/az3/1` for a doppler) |
| tags      | key/value tags to include additional identifying information. (e.g. `deployment=cf-warden`)                                        |


## v2 Envelope Types

#### Log

A *Log* is used to represent a simple text payload.

It represents whether the log is emitted to STDOUT or STDERR.

#### Counter

A *Counter* is used to represent a metric that only increases in value (*e.g.* `metron.sentEnvelopes`).

The emitter of a counter must set the `delta` (anything else will be discarded). It also provides the sum of all emitted values.

#### Gauge

A *Gauge* is used to represent a metric that can have arbitary numeric values that increase or decrease.

It can be used emit a set of relatable metrics (*e.g.* `memory{value=2048, unit=byte}, disk{value=4096, unit=byte}, cpu{value=2, unit=percentage}`)

#### Timer

A *Timer* is used to represent a metric that captures the duration of an event. (*e.g.* `databasePost`)

----

## v1 -> v2 Mapping

The properties in a v1 envelope can be obtained from a v2 envelope using the following mappings:

### Tags

Note previous versions of the Loggregator API automatically added tags to
envelopes for things like `deployment`, `job`, and `index`. This functionality
has been removed in the v2 API. Users should manually add whatever tags they
require.

#### Envelope

| v1         | v2                               |
|------------|----------------------------------|
| timestamp  | envelope.timestamp               |
| tags       | envelope.tags                    |
| origin     | envelope.tags['origin'].text     |
| deployment | envelope.tags['deployment'].text |
| job        | envelope.tags['job'].text        |
| index      | envelope.tags['index'].text      |
| ip         | envelope.tags['ip'].text         |


#### HttpStartStop

An *HttpStartStop* envelope is derived from a v2 *Timer* envelope.

| v1             | v2                                      |
|----------------|-----------------------------------------|
| startTimestamp | timer.start                             |
| stopTimestamp  | timer.stop                              |
| applicationId  | envelope.source_id                      |
| requestId      | envelope.tags['request_id'].text        |
| peerType       | envelope.tags['peer_type'].text         |
| method         | envelope.tags['method'].text            |
| uri            | envelope.tags['uri'].text               |
| remoteAddress  | envelope.tags['remote_address'].text    |
| userAgent      | envelope.tags['user_agent'].text        |
| statusCode     | envelope.tags['status_code'].integer    |
| contentLength  | envelope.tags['content_length'].integer |
| instanceIndex  | envelope.tags['instance_index'].integer |
| instanceId     | envelope.tags['instance_id'].text       |
| forwarded      | envelope.tags['forwarded'].text         |

#### LogMessage

A *LogMessage* envelope is derived from a v2 *Log* envelope

| v1              | v2                                    |
|-----------------|---------------------------------------|
| message         | log.payload                           |
| message_type    | log.type                              |
| timestamp       | envelope.timestamp                    |
| app_id          | envelope.source_id                    |
| source_type     | envelope.tags['source_type'].text     |
| source_instance | envelope.tags['source_instance'].text |

#### CounterEvent

A *CounterEvent* envelope is dervied from a v2 *Counter* envelope

| v1    | v2            |
|-------|---------------|
| name  | counter.name  |
| delta | -             |
| total | counter.total |

#### ValueMetric

A *ValueMetric* envelope is dervied from a v2 *Gauge* envelope if and only if there is a single *Gauge* metric.

| v1    | v2                             |
|-------|--------------------------------|
| name  | first-key                      |
| value | gauge.metrics[first-key].value |
| unit  | gauge.metrics[first-key].unit  |

#### ContainerMetric

A *ContainerMetric* envelope is dervied from a v2 *Gauge* envelope if and only if there are the correct gauge keys.

| v1               | v2                                    |
|------------------|---------------------------------------|
| applicationId    | envelope.source_id                    |
| instanceIndex    | gauge.metrics['instance_index'].value |
| cpuPercentage    | gauge.metrics['cpu'].value            |
| memoryBytes      | gauge.metrics['memory'].value         |
| diskBytes        | gauge.metrics['disk'].value           |
| memoryBytesQuota | gauge.metrics['memory_quota'].value   |
| diskBytesQuota   | gauge.metrics['disk_quota'].value     |
