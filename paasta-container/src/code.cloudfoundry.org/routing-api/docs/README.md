Routing API Documentation
=========================

Reference documentation for client authors using the Routing API.


List HTTP Routes
-------------------
#### Request
  `GET /routing/v1/routes`
#### Request Headers
  A bearer token for an OAuth client with `routing.routes.read` scope is required.

#### Example Request
```sh
curl -vvv -H "Authorization: bearer [uaa token]" http://127.0.0.1:8080/routing/v1/routes
```

#### Expected Response
  Status `200 OK`

#### Expected JSON-Encoded Response Body
  An array of [`HTTP Route`](#http-route) objects.

#### Example Response
```
[{
  "route": "127.0.0.1:8080/routing",
  "port": 3000,
  "ip": "1.2.3.4",
  "ttl": 120,
  "log_guid": "routing_api",
  "modification_tag": {
    "guid": "abc123",
    "index": 1164
  }
}]
```

Register HTTP Routes
-------------------
#### Request
  `POST /routing/v1/routes`
#### Request Headers
  A bearer token for an OAuth client with `routing.routes.write` scope is required.
#### Request JSON-Encoded Body
  An array of `HTTP Route` objects for each route to register.

#### HTTP Route Object
| Object Field        | Type            | Required? | Description |
|---------------------|-----------------|-----------|-------------|
| `route`             | string          | yes       | Address, including optional path, associated with one or more backends
| `ip`                | string          | yes       | IP address of backend                                                   
| `port`              | integer         | yes       | Backend port. Must be greater than 0.
| `ttl`               | integer         | yes       | Time to live, in seconds. The mapping of backend to route will be pruned after this time. It must be greater than 0 seconds and less than 60 seconds.
| `log_guid`          | string          | no        | A string used to annotate routing logs for requests forwarded to this backend.
| `route_service_url` | string          | no        | When present, requests for the route will be forwarded to this url before being forwarded to a backend. If provided, this url must use HTTPS.

#### Example Request
```sh
curl -vvv -H "Authorization: bearer [uaa token]" -X POST http://127.0.0.1:8080/routing/v1/routes -d '[{"ip":"1.2.3.4", "route":"a_route", "port":8089, "ttl":45}]'
```

#### Expected Response
  Status `201 CREATED`


Delete HTTP Routes
-------------------
#### Request
  `DELETE /routing/v1/routes`
#### Request Headers
  A bearer token for an OAuth client with `routing.routes.write` scope is required.
#### Request JSON-Encoded Body
  An array of `HTTP Route` objects for each route to delete.

#### HTTP Route Object
| Object Field        | Type            | Required? | Description |
|---------------------|-----------------|-----------|-------------|
| `route`             | string          | yes       | Address, including optional path, associated with one or more backends
| `ip`                | string          | yes       | IP address of backend
| `port`              | integer         | yes       | Backend port. Must be greater than 0.
| `log_guid`          | string          | no        | A string used to annotate routing logs for requests forwarded to this backend.
| `route_service_url` | string          | no        | When present, requests for the route will be forwarded to this url before being forwarded to a backend. If provided, this url must use HTTPS.

#### Example Request
```sh
curl -vvv -H "Authorization: bearer [uaa token]" -X DELETE http://127.0.0.1:8080/routing/v1/routes -d '[{"ip":"1.2.3.4", "route":"a_route", "port":8089, "ttl":45}]'
```

#### Expected Response
  Status `204 NO CONTENT`


Subscribing to HTTP Route Changes:
-------------------
#### Request
  `GET /routing/v1/events`

#### Request Headers
  A bearer token for an OAuth client with `routing.routes.read` scope is required.

#### Example Request
```sh
curl -vvv -H "Authorization: bearer [uaa token]" http://127.0.0.1:8080/routing/v1/events
```
#### Expected Response
  Status `200 OK`

  The response is a long lived HTTP connection of content type
  `text/event-stream` as defined by
  https://www.w3.org/TR/2012/CR-eventsource-20121211/.

#### Example Response Format:

```
id: 13
event: Upsert
data: {"route":"127.0.0.1:8080/routing","port":3000,"ip":"1.2.3.4","ttl":120,"log_guid":"routing_api","modification_tag":{"guid":"abc123","index":1154}}

id: 14
event: Upsert
data: {"route":"127.0.0.1:8080/routing","port":3000,"ip":"1.2.3.4","ttl":120,"log_guid":"routing_api","modification_tag":{"guid":"abc123","index":1155}}
```

Listing Router Groups
-------------------
#### Request
  `GET /routing/v1/router_groups`

#### Request Headers
  A bearer token for an OAuth client with `routing.router_groups.read` scope is required.

#### Example request
```sh
curl -vvv -H "Authorization: bearer [uaa token]" http://127.0.0.1:8080/routing/v1/router_groups
```

#### Expected Response
  Status `200 OK`

#### Expected JSON-Encoded Response Body
  An array of [`Router Group`](#router-group) objects.

#### Example Response
```
[{
  "guid": "abc123",
  "name": "default-tcp",
  "reservable_ports":"1024-65535"
  "type": "tcp"
}]
```

Update Router Group
-------------------
To update a Router Group's `reservable_ports` field with a new port range.

#### Request
  `PUT /routing/v1/router_groups/:guid`

  `:guid` is the GUID of the router group to be updated.

#### Request Headers
  A bearer token for an OAuth client with `routing.router_groups.write` scope is required.

#### Request JSON-Encoded Body
  A single `Router Group` object for the router group to modify.
  Only the `reservable_ports` field is updated.

#### Router Group Object
| Object Field       | Type   | Required? | Description |
|--------------------|--------|-----------|-------------|
| `reservable_ports` | string | yes       | Comma delimited list of reservable port or port ranges. These ports must fall between 1024 and 65535 (inclusive).

  > **Warning:** If routes are registered for ports that are not in the new range,
  > modifying your load balancer to remove these ports will result in backends for
  > those routes becoming inaccessible.

#### Example Request   
```sh
curl -vvv -H "Authorization: bearer [uaa token]" http://127.0.0.1:8080/routing/v1/router_groups/abc123 -X PUT -d '{"reservable_ports":"9000-10000"}'
```
#### Expected Response
  Status `200 OK`

#### Expected JSON-Encoded Response Body
  The updated [`Router Group`](#router-group).

#### Example Response:
```
{
  "guid": "abc123",
  "name": "default-tcp",
  "reservable_ports":"9000-10000"
  "type": "tcp"
}
```

Register TCP Route
-------------------
#### Sample Request
  `POST /routing/v1/tcp_routes/create`

#### Request Headers
  A bearer token for an OAuth client with `routing.routes.write` scope is required.

#### Request JSON-Encoded Body
  An array of `TCP Route Mapping` objects for each route to register.

#### TCP Route Mapping
| Object Field        | Type            | Required? | Description |
|---------------------|-----------------|-----------|-------------|
| `router_group_guid` | string          | yes       | GUID of the router group associated with this route.
| `port`              | string          | yes       | External facing port for the TCP route.
| `backend_ip`        | integer         | yes       | IP address of backend
| `backend_port`      | string          | yes       | Backend port. Must be greater than 0.
| `ttl`               | integer         | yes       | Time to live, in seconds. The mapping of backend to route will be pruned after this time. Must be greater than 0 seconds and less than 60 seconds.

#### Example Request
```sh
curl -vvv -H "Authorization: bearer [uaa token]" -X POST http://127.0.0.1:8080/routing/v1/tcp_routes/create -d '
[{
  "router_group_guid": "xyz789",
  "port": 5200,
  "backend_ip": "10.1.1.12",
  "backend_port": 60000,
  "ttl": 30
}]'
```

#### Expected Response
  Status `201 CREATED`


Delete TCP Route
-------------------
#### Request
  `POST /routing/v1/tcp_routes/delete`

#### Request Headers
  A bearer token for an OAuth client with `routing.routes.write` scope is required.

#### Request JSON-Encoded Body
  An array of `TCP Route Mapping` objects for each route to delete.

#### TCP Route Mapping
| Object Field        | Type            | Required? | Description |
|---------------------|-----------------|-----------|-------------|
| `router_group_guid` | string          | yes       | GUID of the router group associated with this route.
| `port`              | string          | yes       | External facing port for the TCP route.
| `backend_ip`        | integer         | yes       | IP address of backend
| `backend_port`      | string          | yes       | Backend port. Must be greater than 0.

#### Example Request
```sh
curl -vvv -H "Authorization: bearer [uaa token]" -X POST http://127.0.0.1:8080/routing/v1/tcp_routes/delete -d '
[{
  "router_group_guid": "xyz789",
  "port": 5200,
  "backend_ip": "10.1.1.12",
  "backend_port": 60000
}]'
```

#### Expected Response
  Status `204 NO CONTENT`

List TCP Routes
-------------------
#### Request
  `GET /routing/v1/tcp_routes`

#### Request Headers
  A bearer token for an OAuth client with `routing.routes.read` scope is required.

#### Example Request
```sh
curl -vvv -H "Authorization: bearer [uaa token]" http://127.0.0.1:8080/routing/v1/tcp_routes
```

#### Expected Response
  Status `200 OK`

#### Expected JSON-Encoded Response Body
  An array of [`TCP Route Mapping`](#tcp-route-mapping-2) objects.

#### Example Response:
```
[{
  "router_group_guid": "xyz789",
  "port": 5200,
  "backend_ip": "10.1.1.12",
  "backend_port": 60000
}]
```

Subscribing to TCP Route Changes:
-------------------
#### Request
  `GET /routing/v1/tcp_routes/events`

#### Request Headers
  A bearer token for an OAuth client with `routing.routes.read` scope is required.

#### Example Request
```sh
curl -vvv -H "Authorization: bearer [uaa token]" http://127.0.0.1:8080/routing/v1/tcp_events
```
#### Expected Response
  Status `200 OK`

  The response is a long lived HTTP connection of content type
  `text/event-stream` as defined by
  https://www.w3.org/TR/2012/CR-eventsource-20121211/.

#### Example Response Format:

```
id: 0
event: Upsert
data: {"router_group_guid":"xyz789","port":5200,"backend_port":60000,"backend_ip":"10.1.1.12","modification_tag":{"guid":"abc123","index":1},"ttl":120}

id: 1
event: Upsert
data: {"router_group_guid":"xyz789","port":5200,"backend_port":60000,"backend_ip":"10.1.1.12","modification_tag":{"guid":"abc123","index":2},"ttl":120}
```

-------------------

API Object Types
-------------------

### HTTP Route
| Object Field        | Type            | Description |
|---------------------|-----------------|-------------|
| `route`             | string          | Address, including optional path, associated with one or more backends
| `ip`                | string          | IP address of backend
| `port`              | integer         | Backend port. Must be greater than 0.
| `ttl`               | integer         | Time to live, in seconds. The mapping of backend to route will be pruned after this time.
| `log_guid`          | string          | A string used to annotate routing logs for requests forwarded to this backend.
| `route_service_url` | string          | When present, requests for the route will be forwarded to this url before being forwarded to a backend. If provided, this url must use HTTPS.
| `modification_tag`  | ModificationTag | See [Modification Tags](modification_tags.md).

### Modification Tags
  See [Modification Tags](modification_tags.md).

### TCP Route Mapping
| Object Field        | Type            | Description |
|---------------------|-----------------|-------------|
| `router_group_guid` | string          | GUID of the router group associated with this route.
| `port`              | string          | External facing port for the TCP route.
| `backend_ip`        | integer         | IP address of backend.
| `backend_port`      | string          | Backend port. Must be greater than 0.
| `ttl`               | integer         | Time to live, in seconds. The mapping of backend to route will be pruned after this time.
| `modification_tag`  | ModificationTag | See [Modification Tags](modification_tags.md).

### Router Group
| Object Field       | Type   | Description |
|--------------------|--------|-------------|
| `guid`             | string | GUID of the router group.
| `name`             | string | External facing port for the TCP route.
| `type`             | string | Type of the router group e.g. `tcp`.
| `reservable_ports` | string | Comma delimited list of reservable port or port ranges.
