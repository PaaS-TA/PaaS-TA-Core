package assets

type Assets struct {
	TcpDropletReceiver string
	TcpSampleReceiver  string
	TcpSampleGolang    string
}

func NewAssets() Assets {
	return Assets{
		TcpDropletReceiver: "../assets/tcp-droplet-receiver/",
		TcpSampleReceiver:  "../assets/tcp-sample-receiver/",
		TcpSampleGolang:    "../assets/golang/",
	}
}
