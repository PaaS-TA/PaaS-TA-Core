package stager

import "github.com/tedsuo/rata"

const (
	StageRoute            = "Stage"
	StopStagingRoute      = "StopStaging"
	StagingCompletedRoute = "StagingCompleted"
)

var Routes = rata.Routes{
	{Path: "/v1/staging/:staging_guid", Method: "PUT", Name: StageRoute},
	{Path: "/v1/staging/:staging_guid", Method: "DELETE", Name: StopStagingRoute},
	{Path: "/v1/staging/:staging_guid/completed", Method: "POST", Name: StagingCompletedRoute},
}
