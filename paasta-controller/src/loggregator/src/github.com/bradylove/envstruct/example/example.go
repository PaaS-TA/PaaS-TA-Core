package main

import "github.com/bradylove/envstruct"

type HostInfo struct {
	Ip       string `env:"host_ip,required"`
	Password string `env:"password,noreport"`
	Port     int    `env:"host_port"`
}

func main() {
	hi := HostInfo{Port: 80}

	err := envstruct.Load(&hi)
	if err != nil {
		panic(err)
	}

	envstruct.WriteReport(&hi)
}
