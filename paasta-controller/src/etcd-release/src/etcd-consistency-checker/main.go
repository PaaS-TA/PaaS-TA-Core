package main

import (
	"flag"
	"log"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/etcd-release/src/etcd-consistency-checker/app"
)

type Flags struct {
	ClusterMembers string
	CA             string
	Cert           string
	Key            string
}

func main() {
	flags := Flags{}
	flag.StringVar(&flags.ClusterMembers, "cluster-members", "", "comma seperated list of cluster members")
	flag.StringVar(&flags.CA, "ca-cert", "", "path to the CA Certificate file")
	flag.StringVar(&flags.Cert, "cert", "", "path to the Certificate file")
	flag.StringVar(&flags.Key, "key", "", "path to the Key file")
	flag.Parse()

	a := app.New(app.Config{
		CA:             flags.CA,
		Cert:           flags.Cert,
		Key:            flags.Key,
		ClusterMembers: strings.Split(flags.ClusterMembers, ","),
	},
		time.Sleep,
	)

	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}
