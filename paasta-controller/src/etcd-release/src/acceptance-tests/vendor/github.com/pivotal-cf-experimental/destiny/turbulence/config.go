package turbulence

type ConfigV2 struct {
	Name             string
	AZs              []string
	DirectorHost     string
	DirectorUsername string
	DirectorPassword string
	DirectorCACert   string
}
