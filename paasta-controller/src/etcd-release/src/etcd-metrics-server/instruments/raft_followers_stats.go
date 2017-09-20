package instruments

type RaftFollowersStats struct {
	Leader    string                        `json:"leader"`
	Followers map[string]*RaftFollowerStats `json:"followers"`
}

type RaftFollowerStats struct {
	Latency struct {
		Current           float64 `json:"current"`
		Average           float64 `json:"average"`
		averageSquare     float64
		StandardDeviation float64 `json:"standardDeviation"`
		Minimum           float64 `json:"minimum"`
		Maximum           float64 `json:"maximum"`
	} `json:"latency"`

	Counts struct {
		Fail    uint64 `json:"fail"`
		Success uint64 `json:"success"`
	} `json:"counts"`
}
