package bosh

type Release struct {
	Name     string
	Versions []string
}

func NewRelease() Release {
	return Release{}
}

func (r Release) Latest() string {
	// THIS ASSUMES THE VERSIONS ARE ALREADY SORTED
	return r.Versions[len(r.Versions)-1]
}
