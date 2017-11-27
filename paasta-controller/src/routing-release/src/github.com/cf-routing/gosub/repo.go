package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Repo struct {
	Origin string
	Branch string
}

func getDepRoot(repo string, gopath string, pkg string) (string, *Repo, error) {
	var err error

	pkgRoot := filepath.Join(gopath, "src", pkg)

	root, err := getGitTopLevel(pkgRoot)
	if err != nil {
		return "", nil, err
	}

	rootRoot := root
	for rootRoot != repo && strings.HasPrefix(rootRoot, repo) {
		root = rootRoot
		rootRoot, err = getGitTopLevel(filepath.Dir(root))

		if err != nil {
			return "", nil, err
		}
	}

	if root == repo {
		// non-git repo; point to package instead
		return pkgRoot, nil, nil
	}

	pkgRepo := &Repo{}

	gitOriginURL := exec.Command("git", "config", "--get", "remote.origin.url")
	gitOriginURL.Dir = root

	buf := new(bytes.Buffer)

	gitOriginURL.Stdout = buf
	gitOriginURL.Stderr = os.Stderr

	err = gitOriginURL.Run()
	if err != nil {
		return "", nil, err
	}

	pkgRepo.Origin = strings.TrimRight(buf.String(), "\n")

	gitRevParse := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	gitRevParse.Dir = root

	buf = new(bytes.Buffer)

	gitRevParse.Stdout = buf
	gitRevParse.Stderr = os.Stderr

	err = gitRevParse.Run()
	if err != nil {
		return "", nil, err
	}

	rev := strings.TrimRight(buf.String(), "\n")
	if rev != "HEAD" {
		pkgRepo.Branch = rev
	}

	return root, pkgRepo, nil
}

func getGitTopLevel(path string) (string, error) {
	gitToplevel := exec.Command("git", "rev-parse", "--show-toplevel")
	gitToplevel.Dir = path

	buf := new(bytes.Buffer)

	gitToplevel.Stdout = buf
	gitToplevel.Stderr = os.Stderr

	err := gitToplevel.Run()
	if err != nil {
		return "", err
	}

	return strings.TrimRight(buf.String(), "\n"), nil
}
