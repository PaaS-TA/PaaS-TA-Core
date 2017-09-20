package volman

import "github.com/tedsuo/rata"

const (
	ListDriversRoute = "drivers"
	MountRoute       = "mount"
	UnmountRoute     = "unmount"
)

var Routes = rata.Routes{
	{Path: "/drivers", Method: "GET", Name: ListDriversRoute},
	{Path: "/drivers/mount", Method: "POST", Name: MountRoute},
	{Path: "/drivers/unmount", Method: "POST", Name: UnmountRoute},
}

type ListDriversResponse struct {
	Drivers []InfoResponse `json:"drivers"`
}

type MountRequest struct {
	DriverId string                 `json:"driverId"`
	VolumeId string                 `json:"volumeId"`
	Config   map[string]interface{} `json:"config"`
}

type MountResponse struct {
	Path string `json:"path"`
}

type InfoResponse struct {
	Name string `json:"name"`
}

type UnmountRequest struct {
	DriverId string `json:"driverId"`
	VolumeId string `json:"volumeId"`
}

func NewError(err error) Error {
	return Error{err.Error()}
}

type Error struct {
	Description string `json:"description"`
}

func (e Error) Error() string {
	return e.Description
}
