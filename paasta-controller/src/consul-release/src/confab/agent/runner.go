package agent

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"

	"code.cloudfoundry.org/lager"
)

type Runner struct {
	Path      string
	PIDFile   string
	ConfigDir string
	Stdout    io.Writer
	Stderr    io.Writer
	Recursors []string
	Logger    logger
	cmd       *exec.Cmd
	wg        sync.WaitGroup
	exited    int32
}

func (r *Runner) Run() error {
	if _, err := os.Stat(r.ConfigDir); os.IsNotExist(err) {
		err := fmt.Errorf("config dir does not exist: %s", r.ConfigDir)
		r.Logger.Error("agent-runner.run.config-dir-missing", err)
		return err
	}

	args := []string{
		"agent",
		fmt.Sprintf("-config-dir=%s", r.ConfigDir),
	}

	for _, recursor := range r.Recursors {
		args = append(args, fmt.Sprintf("-recursor=%s", recursor))
	}

	r.cmd = exec.Command(r.Path, args...)
	r.cmd.Stdout = r.Stdout
	r.cmd.Stderr = r.Stderr

	r.Logger.Info("agent-runner.run.start", lager.Data{
		"cmd":  r.Path,
		"args": args,
	})
	err := r.cmd.Start()
	if err != nil {
		r.Logger.Error("agent-runner.run.start.failed", errors.New(err.Error()), lager.Data{
			"cmd":  r.Path,
			"args": args,
		})
		return err
	}

	r.wg.Add(1)
	go func() {
		r.cmd.Wait()
		atomic.StoreInt32(&r.exited, 1)
		r.wg.Done()
	}()

	r.Logger.Info("agent-runner.run.success")
	return nil
}

func (r *Runner) Exited() bool { return atomic.LoadInt32(&r.exited) == 1 }

func (r *Runner) WritePID() error {
	r.Logger.Info("agent-runner.run.write-pidfile", lager.Data{
		"pid":  r.cmd.Process.Pid,
		"path": r.PIDFile,
	})

	if err := ioutil.WriteFile(r.PIDFile, []byte(fmt.Sprintf("%d", r.cmd.Process.Pid)), 0644); err != nil {
		err = fmt.Errorf("error writing PID file: %s", err)
		r.Logger.Error("agent-runner.run.write-pidfile.failed", err, lager.Data{
			"pid":  r.cmd.Process.Pid,
			"path": r.PIDFile,
		})
		return err
	}

	return nil
}

func (r *Runner) getProcess() (*os.Process, error) {
	if r.cmd != nil && r.cmd.Process != nil {
		return r.cmd.Process, nil
	}

	pidFileContents, err := ioutil.ReadFile(r.PIDFile)
	if err != nil {
		return nil, err
	}

	pid, err := strconv.Atoi(string(pidFileContents))
	if err != nil {
		return nil, err
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return nil, err // not tested. As of Go 1.5, FindProcess never errors
	}

	return process, nil
}

func (r *Runner) Wait() error {
	r.Logger.Info("agent-runner.wait.get-process")

	process, err := r.getProcess()
	if err != nil {
		r.Logger.Error("agent-runner.wait.get-process.failed", errors.New(err.Error()))
		return err
	}

	r.Logger.Info("agent-runner.wait.get-process.result", lager.Data{
		"pid": process.Pid,
	})

	r.Logger.Info("agent-runner.wait.signal", lager.Data{
		"pid": process.Pid,
	})

	r.wg.Wait()

	r.Logger.Info("agent-runner.wait.success")
	return nil
}

func (r *Runner) Stop() error {
	r.Logger.Info("agent-runner.stop.get-process")

	process, err := r.getProcess()
	if err != nil {
		r.Logger.Error("agent-runner.stop.get-process.failed", errors.New(err.Error()))
		return err
	}

	r.Logger.Info("agent-runner.stop.get-process.result", lager.Data{
		"pid": process.Pid,
	})

	r.Logger.Info("agent-runner.stop.signal", lager.Data{
		"pid": process.Pid,
	})

	err = process.Signal(syscall.Signal(syscall.SIGKILL))
	if err != nil {
		r.Logger.Error("agent-runner.stop.signal.failed", err)
		return err
	}

	r.Logger.Info("agent-runner.stop.success")
	return nil
}

func (r *Runner) Cleanup() error {
	r.Logger.Info("agent-runner.cleanup.remove", lager.Data{
		"pidfile": r.PIDFile,
	})

	if err := os.Remove(r.PIDFile); err != nil {
		r.Logger.Error("agent-runner.cleanup.remove.failed", errors.New(err.Error()), lager.Data{
			"pidfile": r.PIDFile,
		})
		return err
	}

	r.Logger.Info("agent-runner.cleanup.success")

	return nil
}
