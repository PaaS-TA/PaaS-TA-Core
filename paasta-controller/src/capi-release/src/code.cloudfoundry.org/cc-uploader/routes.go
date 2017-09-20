package ccuploader

import "github.com/tedsuo/rata"

const (
	UploadDropletRoute        = "UploadDroplet"
	UploadBuildArtifactsRoute = "UploadBuildArtifacts"
)

var Routes = rata.Routes{
	{Name: UploadDropletRoute, Method: "POST", Path: "/v1/droplet/:guid"},
	{Name: UploadBuildArtifactsRoute, Method: "POST", Path: "/v1/build_artifacts/:app_guid"},
}
