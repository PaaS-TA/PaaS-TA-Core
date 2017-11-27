package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/codegangsta/cli"
)

func sync(c *cli.Context) error {
	repo := c.String("repo")
	gopath := c.String("gopath")
	ignoredSubmodules := c.StringSlice("ignore")

	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return fmt.Errorf("could not resolve repo: %s", err)
	}

	absGopath, err := filepath.Abs(gopath)
	if err != nil {
		return fmt.Errorf("could not resolve gopath: %s", err)
	}

	pkgRoots := map[string]*Repo{}

	for _, dep := range c.Args() {
		root, repo, err := getDepRoot(absRepo, absGopath, dep)
		if err != nil {
			return fmt.Errorf("failed to get dependency repo: %s", err)
		}

		pkgRoots[root] = repo
	}

	existingSubmodules, err := detectExistingGoSubmodules(repo, gopath, false)
	if err != nil {
		if fixErr := fixExistingSubmodules(repo); fixErr != nil {
			return fmt.Errorf("failed to fix existing submodules: %s", fixErr)
		}
		existingSubmodules, err = detectExistingGoSubmodules(repo, gopath, true)
		if err != nil {
			return fmt.Errorf("failed to detect existing submodules: %s", err)
		}
	}

	gitmodules := filepath.Join(repo, ".gitmodules")

	submodulesToRemove := map[string]bool{}
	for _, submodule := range existingSubmodules {
		submodulesToRemove[submodule] = true
	}

	for _, submodule := range ignoredSubmodules {
		_, exists := submodulesToRemove[submodule]
		if exists {
			delete(submodulesToRemove, submodule)
		}
	}

	for pkgRoot, pkgRepo := range pkgRoots {
		relRoot, err := filepath.Rel(absRepo, pkgRoot)
		if err != nil {
			return fmt.Errorf("could not resolve submodule: %s", err)
		}

		fmt.Println("\x1b[32msyncing " + relRoot + "\x1b[0m")

		// keep this submodule
		delete(submodulesToRemove, relRoot)

		add := exec.Command("git", "add", pkgRoot)
		add.Dir = repo
		add.Stderr = os.Stderr

		err = add.Run()
		if err != nil {
			return fmt.Errorf("error clearing submodule: %s", err)
		}

		if pkgRepo == nil {
			// non-git dependency; vendored
			continue
		}

		status := exec.Command("git", "status", "--porcelain")
		status.Dir = filepath.Join(absRepo, relRoot)

		statusOutput, err := status.Output()
		if err != nil {
			return fmt.Errorf("error fetching submodule status: %s", err)
		}

		if len(statusOutput) != 0 {
			return fmt.Errorf("\x1b[31msubmodule is dirty: " + pkgRoot + "\x1b[0m")
		}

		gitConfig := exec.Command("git", "config", "--file", gitmodules, "submodule."+relRoot+".path", relRoot)
		gitConfig.Stderr = os.Stderr

		err = gitConfig.Run()
		if err != nil {
			return fmt.Errorf("error configuring submodule: %s", err)
		}

		url := httpsOrigin(pkgRepo.Origin)

		if !c.Bool("force-https") {
			gitConfig = exec.Command("git", "config", "--file", gitmodules, "submodule." + relRoot + ".url")
			gitConfig.Stderr = os.Stderr

			out, err := gitConfig.Output()

			if err == nil {
				url = strings.TrimRight(string(out), "\n")
			}
		}

		gitConfig = exec.Command("git", "config", "--file", gitmodules, "submodule."+relRoot+".url", url)
		gitConfig.Stderr = os.Stderr

		err = gitConfig.Run()
		if err != nil {
			return fmt.Errorf("error configuring submodule: %s", err)
		}

		if pkgRepo.Branch != "" {
			gitConfig = exec.Command("git", "config", "--file", gitmodules, "submodule."+relRoot+".branch", pkgRepo.Branch)
			gitConfig.Stderr = os.Stderr

			err = gitConfig.Run()
			if err != nil {
				return fmt.Errorf("error configuring submodule: %s", err)
			}
		}

		gitAdd := exec.Command("git", "add", gitmodules)
		gitAdd.Dir = repo
		gitAdd.Stderr = os.Stderr

		err = gitAdd.Run()
		if err != nil {
			return fmt.Errorf("error staging submodule config: %s", err)
		}
	}

	for submodule, _ := range submodulesToRemove {
		fmt.Println("\x1b[31mremoving " + submodule + "\x1b[0m")

		rm := exec.Command("git", "rm", "--cached", "-f", submodule)
		rm.Dir = repo
		rm.Stderr = os.Stderr

		err := rm.Run()
		if err != nil {
			return fmt.Errorf("error clearing submodule: %s", err)
		}

		gitConfig := exec.Command("git", "config", "--file", gitmodules, "--remove-section", "submodule."+submodule)
		gitConfig.Dir = repo
		gitConfig.Stderr = os.Stderr

		err = gitConfig.Run()
		if err != nil {
			return fmt.Errorf("error removing submodule config: %s", err)
		}

		gitAdd := exec.Command("git", "add", gitmodules)
		gitAdd.Dir = repo
		gitAdd.Stderr = os.Stderr

		err = gitAdd.Run()
		if err != nil {
			return fmt.Errorf("error staging submodule config: %s", err)
		}
	}

	if err := fixExistingSubmodules(repo); err != nil {
		return fmt.Errorf("failed to fix submodules: %s", err)
	}

	return nil
}

func detectExistingGoSubmodules(repo string, gopath string, printErrors bool) ([]string, error) {
	srcPath := filepath.Join(gopath, "src")

	submoduleStatus := exec.Command("git", "submodule", "status", srcPath)
	submoduleStatus.Dir = repo

	if printErrors {
		submoduleStatus.Stderr = os.Stderr
	}

	statusOut, err := submoduleStatus.StdoutPipe()
	if err != nil {
		printErr(printErrors, "detectExistingGoSubmodules failed to get StdoutPipe: %s\n", err)
		return nil, err
	}

	lineScanner := bufio.NewScanner(statusOut)

	err = submoduleStatus.Start()
	if err != nil {
		printErr(printErrors, "detectExistingGoSubmodules failed to start git submodule status: %s\n", err)
		return nil, err
	}

	submodules := []string{}
	for lineScanner.Scan() {
		segments := strings.Split(lineScanner.Text()[1:], " ")

		if len(segments) < 2 {
			return nil, fmt.Errorf("invalid git status output: %q", lineScanner.Text())
		}

		submodules = append(submodules, segments[1])
	}

	err = submoduleStatus.Wait()
	if err != nil {
		printErr(printErrors, "detectExistingGoSubmodules failed to wait for git submodule status: %s\n", err)
		return nil, err
	}

	return submodules, nil
}

func printErr(print bool, format string, err error) {
	if print {
		fmt.Printf(format, err)
	}
}

var sshGitURIRegexp = regexp.MustCompile(`(git@github.com:|https?://github.com/)([^/]+)/(.*?)(\.git)?$`)

func httpsOrigin(uri string) string {
	return sshGitURIRegexp.ReplaceAllString(uri, "https://github.com/$2/$3")
}
