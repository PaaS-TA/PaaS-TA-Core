package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"io/ioutil"
	"path/filepath"

	"github.com/codegangsta/cli"
)

func fix(c *cli.Context) error {
	repo := c.String("repo")

	if fixErr := fixExistingSubmodules(repo); fixErr != nil {
		return fmt.Errorf("failed to fix existing submodules: %s", fixErr)
	}
	return nil
}

// Convert any "semi-submodules" into first class submodules.
// See http://stackoverflow.com/questions/4161022/git-how-to-track-untracked-content/4162672#4162672
func fixExistingSubmodules(repo string) error {
	submodules, err := getSubmodules(repo)
	if err != nil {
		return err
	}

	gitmodules, err := getGitModules(repo)
	if err != nil {
		return err
	}

	var lastErr error
	for _, submodule := range submodules {
		if !gitmodules[submodule] {
			err = fixSubmodule(submodule)
			if err != nil {
				fmt.Printf("fixExistingSubmodules failed to fix submodule %s\n", submodule)
				lastErr = err
			}
		}
	}

	return lastErr
}

func fixSubmodule(submodule string) error {
	fmt.Printf("\x1b[32mFixing submodule %s .", submodule)
	defer fmt.Println("\x1b[0m")
	rm := exec.Command("git", "rm", "--cached", "-f", submodule)
	rm.Stderr = os.Stderr
	err := rm.Run()
	if err != nil {
		return fmt.Errorf("fixSubmodule failed to remove submodule path %s from the index: %s", submodule, err)
	}
	fmt.Printf(".")

	url, err := submoduleUrl(submodule)
	if err != nil {
		return fmt.Errorf("fixSubmodule failed to determine URL of submodule %s: %s", submodule, err)
	}
	fmt.Printf(".")

	submoduleAdd := exec.Command("git", "submodule", "add", url, submodule)
	submoduleAdd.Stderr = os.Stderr
	err = submoduleAdd.Run()
	if err != nil {
		return fmt.Errorf("fixSubmodule failed to add submodule %s: %s", submodule, err)
	}
	fmt.Printf(".")

	fmt.Printf(". done.")
	return nil
}

func submoduleUrl(submodule string) (string, error) {
	submoduleQuery := exec.Command("git", "remote", "show", "origin")
	submoduleQuery.Dir = submodule
	submoduleQuery.Stderr = os.Stderr

	lsFileOut, err := submoduleQuery.StdoutPipe()
	if err != nil {
		fmt.Printf("submoduleUrl failed to get StdoutPipe: %s\n", err)
		return "", err
	}

	lineScanner := bufio.NewScanner(lsFileOut)

	err = submoduleQuery.Start()
	if err != nil {
		fmt.Printf("submoduleUrl failed to start git remote show origin: %s\n", err)
		return "", err
	}

	var url string
	for lineScanner.Scan() {
		segments := strings.Fields(lineScanner.Text())

		if len(segments) < 3 {
			continue
		}

		if segments[0] == "Fetch" && segments[1] == "URL:" {
			url = segments[2]
		}
	}

	if url == "" {
		return "", fmt.Errorf("submoduleUrl failed to find the URL of %s\n", submodule)
	}

	err = submoduleQuery.Wait()
	if err != nil {
		fmt.Printf("submoduleUrl failed to wait for git remote show origin: %s\n", err)
		return "", err
	}

	return url, nil
}

func getSubmodules(repo string) ([]string, error) {
	lsFiles := exec.Command("git", "ls-files", "--stage")
	lsFiles.Dir = repo
	lsFiles.Stderr = os.Stderr

	lsFileOut, err := lsFiles.StdoutPipe()
	if err != nil {
		fmt.Printf("getSubmodules failed to get StdoutPipe: %s\n", err)
		return nil, err
	}

	lineScanner := bufio.NewScanner(lsFileOut)

	err = lsFiles.Start()
	if err != nil {
		fmt.Printf("getSubmodules failed to start git ls-files --stage: %s\n", err)
		return nil, err
	}

	submodules := []string{}
	for lineScanner.Scan() {
		segments := strings.Fields(lineScanner.Text())

		if len(segments) < 4 {
			return nil, fmt.Errorf("invalid git ls-files output: %q", lineScanner.Text())
		}

		if segments[0] == "160000" {
			submodules = append(submodules, segments[3])
		}
	}

	err = lsFiles.Wait()
	if err != nil {
		fmt.Printf("getSubmodules failed to wait for git ls-files --stage: %s\n", err)
		return nil, err
	}

	return submodules, nil
}

func getGitModules(repo string) (map[string]bool, error) {
	gitmodules := make(map[string]bool)

	dotGitModules, err := ioutil.ReadFile(filepath.Join(repo, ".gitmodules"))
	if err != nil {
		return nil, fmt.Errorf("Failed to read .gitmodules file: %s", err)
	}

	lineScanner := bufio.NewScanner(strings.NewReader(string(dotGitModules)))
	for lineScanner.Scan() {
		segments := strings.Fields(lineScanner.Text())
		if len(segments) == 3 && segments[0] == "path" && segments[1] == "=" {
			gitmodules[segments[2]] = true
		}
	}

	return gitmodules, nil
}
