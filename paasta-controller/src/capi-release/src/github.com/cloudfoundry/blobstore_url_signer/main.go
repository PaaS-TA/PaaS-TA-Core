package main

import (
	"flag"
	"os"

	"github.com/cloudfoundry/blobstore_url_signer/server"
	"github.com/cloudfoundry/blobstore_url_signer/signer"
)

var (
	flagBlobstoreSecret string
	flagNetwork         string
	flagLocalAddress    string
)

func main() {
	flag.StringVar(&flagBlobstoreSecret, "secret", "", "The secret for signing webdav url")
	flag.StringVar(&flagNetwork, "network", "", "Network type")
	flag.StringVar(&flagLocalAddress, "laddr", "", "Local network address")
	flag.Parse()

	urlSigner := signer.NewSigner(flagBlobstoreSecret)
	serverHandlers := server.NewServerHandlers(urlSigner)

	if flagNetwork == "unix" {
		os.Remove(flagLocalAddress)
	}

	s := server.NewServer(flagNetwork, flagLocalAddress, serverHandlers)
	s.Start()
}
