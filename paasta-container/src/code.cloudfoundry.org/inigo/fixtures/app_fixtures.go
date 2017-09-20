package fixtures

import (
	"io/ioutil"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func GoServerApp() []archive_helper.ArchiveFile {
	serverPath, err := gexec.Build("code.cloudfoundry.org/inigo/fixtures/go-server")
	Expect(err).NotTo(HaveOccurred())

	contents, err := ioutil.ReadFile(serverPath)
	Expect(err).NotTo(HaveOccurred())
	return []archive_helper.ArchiveFile{
		{
			Name: "go-server",
			Body: string(contents),
		}, {
			Name: "staging_info.yml",
			Body: `detected_buildpack: Doesn't Matter
start_command: go-server`,
		},
	}
}

func CurlLRP() []archive_helper.ArchiveFile {
	return []archive_helper.ArchiveFile{
		{
			Name: "server.sh",
			Body: `#!/bin/bash

kill_app() {
	kill -9 $child
	exit
}

trap kill_app 15 9

mkfifo request

while true; do
	{
		read < request

		echo -n -e "HTTP/1.1 200 OK\r\n"
		echo -n -e "\r\n"
		curl -s --connect-timeout 5 http://www.example.com -o /dev/null ; echo -n $?
	} | nc -l 0.0.0.0 $PORT > request &

	child=$!
	wait $child
done
`,
		},
	}
}
