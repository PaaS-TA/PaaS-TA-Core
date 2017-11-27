package rep

import "github.com/tedsuo/rata"

const (
	StateRoute   = "STATE"
	PerformRoute = "PERFORM"

	StopLRPInstanceRoute = "StopLRPInstance"
	CancelTaskRoute      = "CancelTask"

	Sim_ResetRoute = "RESET"

	PingRoute     = "Ping"
	EvacuateRoute = "Evacuate"

	//Newly Added for container metrics
	ContainerListRoute   = "ContainerList"
)

func NewRoutes(secure bool) rata.Routes {
	var routes rata.Routes

	if secure {
		routes = append(routes,
			rata.Route{Path: "/state", Method: "GET", Name: StateRoute},
			rata.Route{Path: "/work", Method: "POST", Name: PerformRoute},

			rata.Route{Path: "/v1/lrps/:process_guid/instances/:instance_guid/stop", Method: "POST", Name: StopLRPInstanceRoute},
			rata.Route{Path: "/v1/tasks/:task_guid/cancel", Method: "POST", Name: CancelTaskRoute},

			rata.Route{Path: "/sim/reset", Method: "POST", Name: Sim_ResetRoute},
		)
	}

	if !secure {
		routes = append(routes,
			rata.Route{Path: "/ping", Method: "GET", Name: PingRoute},
			rata.Route{Path: "/evacuate", Method: "POST", Name: EvacuateRoute},
			//===============================================================
			//Added : Get Container List
			rata.Route{Path: "/v1/containers", Method:"GET", Name:ContainerListRoute},
			//===============================================================
		)
	}
	return routes

}

var RoutesInsecure = NewRoutes(false)
var RoutesSecure = NewRoutes(true)
var Routes = append(RoutesInsecure, RoutesSecure...)
