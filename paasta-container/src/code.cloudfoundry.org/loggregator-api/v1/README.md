
The first version of loggregator's serialization format did not use gRPC. It
was transported over a mix of websockets, multipart/http, custom tcp, and udp.

If you need to generate code to parse or generate this format you can find the
[protobuf definitions here][dropsonde-protocol].

[dropsonde-protocol]: https://github.com/cloudfoundry/dropsonde-protocol/tree/master/events
