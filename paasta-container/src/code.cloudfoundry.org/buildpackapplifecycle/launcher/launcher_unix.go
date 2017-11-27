// +build !windows

package main

import (
	"os"
	"syscall"
)

const launcher = `
cd "$1"

if [ -d .profile.d ]; then
  for env_file in .profile.d/*; do
    source $env_file
  done
fi

if [ -f .profile ]; then
  source .profile
fi

shift

exec bash -c "$@"
`

func runProcess(dir, command string) {
	syscall.Exec("/bin/bash", []string{
		"bash",
		"-c",
		launcher,
		os.Args[0],
		dir,
		command,
	}, os.Environ())
}
