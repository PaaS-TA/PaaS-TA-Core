package volman

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
