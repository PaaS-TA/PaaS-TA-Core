package buildpackrunner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/buildpackapplifecycle"
	"code.cloudfoundry.org/bytefmt"
	"github.com/cloudfoundry-incubator/candiedyaml"
)

const DOWNLOAD_TIMEOUT = 10 * time.Minute

type Runner struct {
	config *buildpackapplifecycle.LifecycleBuilderConfig
}

type descriptiveError struct {
	message string
	err     error
}

func (e descriptiveError) Error() string {
	if e.err == nil {
		return e.message
	}
	return fmt.Sprintf("%s: %s", e.message, e.err.Error())
}

func newDescriptiveError(err error, message string, args ...interface{}) error {
	if len(args) == 0 {
		return descriptiveError{message: message, err: err}
	}
	return descriptiveError{message: fmt.Sprintf(message, args...), err: err}
}

type Release struct {
	DefaultProcessTypes buildpackapplifecycle.ProcessTypes `yaml:"default_process_types"`
}

func New(config *buildpackapplifecycle.LifecycleBuilderConfig) *Runner {
	return &Runner{
		config: config,
	}
}

func (runner *Runner) Run() error {
	//set up the world
	err := runner.makeDirectories()
	if err != nil {
		return newDescriptiveError(err, "Failed to set up filesystem when generating droplet")
	}

	err = runner.downloadBuildpacks()
	if err != nil {
		return err
	}

	//detect, compile, release
	detectedBuildpack, detectedBuildpackDir, detectOutput, ok := runner.detect()
	if !ok {
		return newDescriptiveError(nil, buildpackapplifecycle.DetectFailMsg)
	}

	err = runner.compile(detectedBuildpackDir)
	if err != nil {
		return newDescriptiveError(nil, buildpackapplifecycle.CompileFailMsg)
	}

	startCommands, err := runner.readProcfile()
	if err != nil {
		return newDescriptiveError(err, "Failed to read command from Procfile")
	}

	releaseInfo, err := runner.release(detectedBuildpackDir, startCommands)
	if err != nil {
		return newDescriptiveError(err, buildpackapplifecycle.ReleaseFailMsg)
	}

	if releaseInfo.DefaultProcessTypes["web"] == "" {
		printError("No start command specified by buildpack or via Procfile.")
		printError("App will not start unless a command is provided at runtime.")
	}

	tarPath, err := exec.LookPath("tar")
	if err != nil {
		return err
	}

	//prepare the final droplet directory
	contentsDir, err := ioutil.TempDir("", "contents")
	if err != nil {
		return newDescriptiveError(err, "Failed to create droplet contents dir")
	}

	//generate staging_info.yml and result json file
	infoFilePath := path.Join(contentsDir, "staging_info.yml")
	err = runner.saveInfo(infoFilePath, detectedBuildpack, detectOutput, releaseInfo)
	if err != nil {
		return newDescriptiveError(err, "Failed to encode generated metadata")
	}

	appDir := path.Join(contentsDir, "app")
	err = runner.copyApp(runner.config.BuildDir(), appDir)
	if err != nil {
		return newDescriptiveError(err, "Failed to copy compiled droplet")
	}

	err = os.MkdirAll(path.Join(contentsDir, "tmp"), 0755)
	if err != nil {
		return newDescriptiveError(err, "Failed to set up droplet filesystem")
	}

	err = os.MkdirAll(path.Join(contentsDir, "logs"), 0755)
	if err != nil {
		return newDescriptiveError(err, "Failed to set up droplet filesystem")
	}

	err = exec.Command(tarPath, "-czf", runner.config.OutputDroplet(), "-C", contentsDir, ".").Run()
	if err != nil {
		return newDescriptiveError(err, "Failed to compress droplet")
	}

	//prepare the build artifacts cache output directory
	err = os.MkdirAll(filepath.Dir(runner.config.OutputBuildArtifactsCache()), 0755)
	if err != nil {
		return newDescriptiveError(err, "Failed to create output build artifacts cache dir")
	}

	err = exec.Command(tarPath, "-czf", runner.config.OutputBuildArtifactsCache(), "-C", runner.config.BuildArtifactsCacheDir(), ".").Run()
	if err != nil {
		return newDescriptiveError(err, "Failed to compress build artifacts")
	}

	return nil
}

func (runner *Runner) makeDirectories() error {
	if err := os.MkdirAll(filepath.Dir(runner.config.OutputDroplet()), 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(runner.config.OutputMetadata()), 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(runner.config.BuildArtifactsCacheDir(), 0755); err != nil {
		return err
	}

	return nil
}

func (runner *Runner) downloadBuildpacks() error {
	// Do we have a custom buildpack?
	for _, buildpackName := range runner.config.BuildpackOrder() {
		buildpackUrl, err := url.Parse(buildpackName)
		if err != nil {
			return fmt.Errorf("Invalid buildpack url (%s): %s", buildpackName, err.Error())
		}
		if !buildpackUrl.IsAbs() {
			continue
		}

		destination := runner.config.BuildpackPath(buildpackName)

		if IsZipFile(buildpackUrl.Path) {
			var size uint64
			zipDownloader := NewZipDownloader(runner.config.SkipCertVerify())
			size, err = zipDownloader.DownloadAndExtract(buildpackUrl, destination)
			if err == nil {
				fmt.Printf("Downloaded buildpack `%s` (%s)", buildpackUrl.String(), bytefmt.ByteSize(size))
			}
		} else {
			err = GitClone(*buildpackUrl, destination)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (runner *Runner) buildpackPath(buildpack string) (string, error) {
	buildpackPath := runner.config.BuildpackPath(buildpack)

	if runner.pathHasBinDirectory(buildpackPath) {
		return buildpackPath, nil
	}

	files, err := ioutil.ReadDir(buildpackPath)
	if err != nil {
		return "", newDescriptiveError(nil, "Failed to read buildpack directory '%s' for buildpack '%s'", buildpackPath, buildpack)
	}

	if len(files) == 1 {
		nestedPath := path.Join(buildpackPath, files[0].Name())

		if runner.pathHasBinDirectory(nestedPath) {
			return nestedPath, nil
		}
	}

	return "", newDescriptiveError(nil, "malformed buildpack does not contain a /bin dir: %s", buildpack)
}

func (runner *Runner) pathHasBinDirectory(pathToTest string) bool {
	_, err := os.Stat(path.Join(pathToTest, "bin"))
	return err == nil
}

// returns buildpack name,  buildpack path, buildpack detect output, ok
func (runner *Runner) detect() (string, string, string, bool) {
	for _, buildpack := range runner.config.BuildpackOrder() {

		buildpackPath, err := runner.buildpackPath(buildpack)
		if err != nil {
			printError(err.Error())
			continue
		}

		if runner.config.SkipDetect() {
			return buildpack, buildpackPath, "", true
		}

		output := new(bytes.Buffer)
		err = runner.run(exec.Command(path.Join(buildpackPath, "bin", "detect"), runner.config.BuildDir()), output)

		if err == nil {
			return buildpack, buildpackPath, strings.TrimRight(output.String(), "\n"), true
		}
	}

	return "", "", "", false
}

func (runner *Runner) readProcfile() (map[string]string, error) {
	processes := map[string]string{}

	procFile, err := os.Open(filepath.Join(runner.config.BuildDir(), "Procfile"))
	if err != nil {
		if os.IsNotExist(err) {
			// Procfiles are optional
			return processes, nil
		}

		return processes, err
	}

	defer procFile.Close()

	err = candiedyaml.NewDecoder(procFile).Decode(&processes)
	if err != nil {
		// clobber candiedyaml's super low-level error
		return processes, errors.New("invalid YAML")
	}

	return processes, nil
}

func (runner *Runner) compile(buildpackDir string) error {
	return runner.run(exec.Command(path.Join(buildpackDir, "bin", "compile"), runner.config.BuildDir(), runner.config.BuildArtifactsCacheDir()), os.Stdout)
}

func (runner *Runner) release(buildpackDir string, startCommands map[string]string) (Release, error) {
	output := new(bytes.Buffer)

	err := runner.run(exec.Command(path.Join(buildpackDir, "bin", "release"), runner.config.BuildDir()), output)
	if err != nil {
		return Release{}, err
	}

	decoder := candiedyaml.NewDecoder(output)

	parsedRelease := Release{}

	err = decoder.Decode(&parsedRelease)
	if err != nil {
		return Release{}, newDescriptiveError(err, "buildpack's release output invalid")
	}

	if len(startCommands) > 0 {
		parsedRelease.DefaultProcessTypes = startCommands
	}

	return parsedRelease, nil
}

func (runner *Runner) saveInfo(infoFilePath, buildpack, detectOutput string, releaseInfo Release) error {
	deaInfoFile, err := os.Create(infoFilePath)
	if err != nil {
		return err
	}
	defer deaInfoFile.Close()

	// JSON ⊂ YAML
	err = json.NewEncoder(deaInfoFile).Encode(DeaStagingInfo{
		DetectedBuildpack: detectOutput,
		StartCommand:      releaseInfo.DefaultProcessTypes["web"],
	})
	if err != nil {
		return err
	}

	resultFile, err := os.Create(runner.config.OutputMetadata())
	if err != nil {
		return err
	}
	defer resultFile.Close()

	err = json.NewEncoder(resultFile).Encode(buildpackapplifecycle.NewStagingResult(
		releaseInfo.DefaultProcessTypes,
		buildpackapplifecycle.LifecycleMetadata{
			BuildpackKey:      buildpack,
			DetectedBuildpack: detectOutput,
		},
	))
	if err != nil {
		return err
	}

	return nil
}

func (runner *Runner) copyApp(buildDir, stageDir string) error {
	return runner.run(exec.Command("cp", "-a", buildDir, stageDir), os.Stdout)
}

func (runner *Runner) run(cmd *exec.Cmd, output io.Writer) error {
	cmd.Stdout = output
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func printError(message string) {
	println(message)
}
