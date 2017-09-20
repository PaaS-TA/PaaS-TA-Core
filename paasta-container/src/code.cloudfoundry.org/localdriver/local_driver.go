package localdriver

import (
	"errors"
	"fmt"
	"os"

	"strings"

	"path/filepath"

	"code.cloudfoundry.org/goshims/filepath"
	"code.cloudfoundry.org/goshims/os"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
	"golang.org/x/crypto/bcrypt"
	"syscall"
)

const VolumesRootDir = "_volumes"
const MountsRootDir = "_mounts"

type LocalVolumeInfo struct {
	passcode []byte

	voldriver.VolumeInfo // see voldriver.resources.go
}

type LocalDriver struct {
	volumes       map[string]*LocalVolumeInfo
	os            osshim.Os
	filepath      filepathshim.Filepath
	mountPathRoot string
}

func NewLocalDriver(os osshim.Os, filepath filepathshim.Filepath, mountPathRoot string) *LocalDriver {
	return &LocalDriver{
		volumes:       map[string]*LocalVolumeInfo{},
		os:            os,
		filepath:      filepath,
		mountPathRoot: mountPathRoot,
	}
}

func (d *LocalDriver) Activate(env voldriver.Env) voldriver.ActivateResponse {
	return voldriver.ActivateResponse{
		Implements: []string{"VolumeDriver"},
	}
}

func (d *LocalDriver) Create(env voldriver.Env, createRequest voldriver.CreateRequest) voldriver.ErrorResponse {
	logger := env.Logger().Session("create")
	var ok bool
	if createRequest.Name == "" {
		return voldriver.ErrorResponse{Err: "Missing mandatory 'volume_name'"}
	}

	var existingVolume *LocalVolumeInfo
	if existingVolume, ok = d.volumes[createRequest.Name]; !ok {
		logger.Info("creating-volume", lager.Data{"volume_name": createRequest.Name, "volume_id": createRequest.Name})

		volInfo := LocalVolumeInfo{VolumeInfo: voldriver.VolumeInfo{Name: createRequest.Name}}
		if passcode, ok := createRequest.Opts["passcode"]; ok {
			if passcodeAsString, ok := passcode.(string); !ok {
				return voldriver.ErrorResponse{Err: "Opts.passcode must be a string value"}
			} else {
				passhash, err := bcrypt.GenerateFromPassword([]byte(passcodeAsString), bcrypt.DefaultCost)
				if err != nil {
					return voldriver.ErrorResponse{Err: "System Failure"}
				}
				volInfo.passcode = passhash
			}
		}
		d.volumes[createRequest.Name] = &volInfo

		createDir := d.volumePath(logger, createRequest.Name)
		logger.Info("creating-volume-folder", lager.Data{"volume": createDir})
		orig := syscall.Umask(000)
		defer syscall.Umask(orig)
		d.os.MkdirAll(createDir, os.ModePerm)

		return voldriver.ErrorResponse{}
	}

	if existingVolume.Name != createRequest.Name {
		logger.Info("duplicate-volume", lager.Data{"volume_name": createRequest.Name})
		return voldriver.ErrorResponse{Err: fmt.Sprintf("Volume '%s' already exists with a different volume ID", createRequest.Name)}
	}

	return voldriver.ErrorResponse{}
}

func (d *LocalDriver) List(env voldriver.Env) voldriver.ListResponse {
	listResponse := voldriver.ListResponse{}
	for _, volume := range d.volumes {
		listResponse.Volumes = append(listResponse.Volumes, volume.VolumeInfo)
	}
	listResponse.Err = ""
	return listResponse
}

func (d *LocalDriver) Mount(env voldriver.Env, mountRequest voldriver.MountRequest) voldriver.MountResponse {
	logger := env.Logger().Session("mount", lager.Data{"volume": mountRequest.Name})

	if mountRequest.Name == "" {
		return voldriver.MountResponse{Err: "Missing mandatory 'volume_name'"}
	}

	var vol *LocalVolumeInfo
	var ok bool
	if vol, ok = d.volumes[mountRequest.Name]; !ok {
		return voldriver.MountResponse{Err: fmt.Sprintf("Volume '%s' must be created before being mounted", mountRequest.Name)}
	}

	if vol.passcode != nil {
		//var hash []bytes
		if passcode, ok := mountRequest.Opts["passcode"]; !ok {
			logger.Info("missing-passcode", lager.Data{"volume_name": mountRequest.Name})
			return voldriver.MountResponse{Err: "Volume " + mountRequest.Name + " requires a passcode"}
		} else {
			if passcodeAsString, ok := passcode.(string); !ok {
				return voldriver.MountResponse{Err: "Opts.passcode must be a string value"}
			} else {
				if bcrypt.CompareHashAndPassword(vol.passcode, []byte(passcodeAsString)) != nil {
					return voldriver.MountResponse{Err: "Volume " + mountRequest.Name + " access denied"}
				}
			}

		}
	}

	volumePath := d.volumePath(logger, vol.Name)

	mountPath := d.mountPath(logger, vol.Name)
	logger.Info("mounting-volume", lager.Data{"id": vol.Name, "mountpoint": mountPath})

	if vol.MountCount < 1 {
		err := d.mount(logger, volumePath, mountPath)
		if err != nil {
			logger.Error("mount-volume-failed", err)
			return voldriver.MountResponse{Err: fmt.Sprintf("Error mounting volume: %s", err.Error())}
		}
		vol.Mountpoint = mountPath
	}

	vol.MountCount++
	logger.Info("volume-mounted", lager.Data{"name": vol.Name, "count": vol.MountCount})

	mountResponse := voldriver.MountResponse{Mountpoint: vol.Mountpoint}
	return mountResponse
}

func (d *LocalDriver) Path(env voldriver.Env, pathRequest voldriver.PathRequest) voldriver.PathResponse {
	logger := env.Logger().Session("path", lager.Data{"volume": pathRequest.Name})

	if pathRequest.Name == "" {
		return voldriver.PathResponse{Err: "Missing mandatory 'volume_name'"}
	}

	mountPath, err := d.get(logger, pathRequest.Name)
	if err != nil {
		logger.Error("failed-no-such-volume-found", err, lager.Data{"mountpoint": mountPath})

		return voldriver.PathResponse{Err: fmt.Sprintf("Volume '%s' not found", pathRequest.Name)}
	}

	if mountPath == "" {
		errText := "Volume not previously mounted"
		logger.Error("failed-mountpoint-not-assigned", errors.New(errText))
		return voldriver.PathResponse{Err: errText}
	}

	return voldriver.PathResponse{Mountpoint: mountPath}
}

func (d *LocalDriver) Unmount(env voldriver.Env, unmountRequest voldriver.UnmountRequest) voldriver.ErrorResponse {
	logger := env.Logger().Session("unmount", lager.Data{"volume": unmountRequest.Name})

	if unmountRequest.Name == "" {
		return voldriver.ErrorResponse{Err: "Missing mandatory 'volume_name'"}
	}

	mountPath, err := d.get(logger, unmountRequest.Name)
	if err != nil {
		logger.Error("failed-no-such-volume-found", err, lager.Data{"mountpoint": mountPath})

		return voldriver.ErrorResponse{Err: fmt.Sprintf("Volume '%s' not found", unmountRequest.Name)}
	}

	if mountPath == "" {
		errText := "Volume not previously mounted"
		logger.Error("failed-mountpoint-not-assigned", errors.New(errText))
		return voldriver.ErrorResponse{Err: errText}
	}

	return d.unmount(logger, unmountRequest.Name, mountPath)
}

func (d *LocalDriver) Remove(env voldriver.Env, removeRequest voldriver.RemoveRequest) voldriver.ErrorResponse {
	logger := env.Logger().Session("remove", lager.Data{"volume": removeRequest})
	logger.Info("start")
	defer logger.Info("end")

	if removeRequest.Name == "" {
		return voldriver.ErrorResponse{Err: "Missing mandatory 'volume_name'"}
	}

	var response voldriver.ErrorResponse
	var vol *LocalVolumeInfo
	var exists bool
	if vol, exists = d.volumes[removeRequest.Name]; !exists {
		logger.Error("failed-volume-removal", fmt.Errorf(fmt.Sprintf("Volume %s not found", removeRequest.Name)))
		return voldriver.ErrorResponse{fmt.Sprintf("Volume '%s' not found", removeRequest.Name)}
	}

	if vol.Mountpoint != "" {
		response = d.unmount(logger, removeRequest.Name, vol.Mountpoint)
		if response.Err != "" {
			return response
		}
	}

	volumePath := d.volumePath(logger, vol.Name)

	logger.Info("remove-volume-folder", lager.Data{"volume": volumePath})
	err := d.os.RemoveAll(volumePath)
	if err != nil {
		logger.Error("failed-removing-volume", err)
		return voldriver.ErrorResponse{Err: fmt.Sprintf("Failed removing mount path: %s", err)}
	}

	logger.Info("removing-volume", lager.Data{"name": removeRequest.Name})
	delete(d.volumes, removeRequest.Name)
	return voldriver.ErrorResponse{}
}

func (d *LocalDriver) Get(env voldriver.Env, getRequest voldriver.GetRequest) voldriver.GetResponse {
	logger := env.Logger().Session("Get")
	mountpoint, err := d.get(logger, getRequest.Name)
	if err != nil {
		return voldriver.GetResponse{Err: err.Error()}
	}

	return voldriver.GetResponse{Volume: voldriver.VolumeInfo{Name: getRequest.Name, Mountpoint: mountpoint}}
}

func (d *LocalDriver) get(logger lager.Logger, volumeName string) (string, error) {
	if vol, ok := d.volumes[volumeName]; ok {
		logger.Info("getting-volume", lager.Data{"name": volumeName})
		return vol.Mountpoint, nil
	}

	return "", errors.New("Volume not found")
}

func (d *LocalDriver) Capabilities(env voldriver.Env) voldriver.CapabilitiesResponse {
	return voldriver.CapabilitiesResponse{
		Capabilities: voldriver.CapabilityInfo{Scope: "local"},
	}
}

func (d *LocalDriver) exists(path string) (bool, error) {
	_, err := d.os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func (d *LocalDriver) mountPath(logger lager.Logger, volumeId string) string {
	dir, err := d.filepath.Abs(d.mountPathRoot)
	if err != nil {
		logger.Fatal("abs-failed", err)
	}

	if !strings.HasSuffix(dir, "/") {
		dir = fmt.Sprintf("%s/", dir)
	}

	mountsPathRoot := fmt.Sprintf("%s%s", dir, MountsRootDir)
	orig := syscall.Umask(000)
	defer syscall.Umask(orig)
	d.os.MkdirAll(mountsPathRoot, os.ModePerm)

	return fmt.Sprintf("%s/%s", mountsPathRoot, volumeId)
}

func (d *LocalDriver) volumePath(logger lager.Logger, volumeId string) string {
	dir, err := d.filepath.Abs(d.mountPathRoot)
	if err != nil {
		logger.Fatal("abs-failed", err)
	}

	volumesPathRoot := filepath.Join(dir, VolumesRootDir)
	orig := syscall.Umask(000)
	defer syscall.Umask(orig)
	d.os.MkdirAll(volumesPathRoot, os.ModePerm)

	return filepath.Join(volumesPathRoot, volumeId)
}

func (d *LocalDriver) mount(logger lager.Logger, volumePath, mountPath string) error {
	logger.Info("link", lager.Data{"src": volumePath, "tgt": mountPath})
	orig := syscall.Umask(000)
	defer syscall.Umask(orig)
	return d.os.Symlink(volumePath, mountPath)
}

func (d *LocalDriver) unmount(logger lager.Logger, name string, mountPath string) voldriver.ErrorResponse {
	logger = logger.Session("unmount")
	logger.Info("start")
	defer logger.Info("end")

	exists, err := d.exists(mountPath)
	if err != nil {
		logger.Error("failed-retrieving-mount-info", err, lager.Data{"mountpoint": mountPath})
		return voldriver.ErrorResponse{Err: "Error establishing whether volume exists"}
	}

	if !exists {
		errText := fmt.Sprintf("Volume %s does not exist (path: %s), nothing to do!", name, mountPath)
		logger.Error("failed-mountpoint-not-found", errors.New(errText))
		return voldriver.ErrorResponse{Err: errText}
	}

	d.volumes[name].MountCount--
	if d.volumes[name].MountCount > 0 {
		logger.Info("volume-still-in-use", lager.Data{"name": name, "count": d.volumes[name].MountCount})
		return voldriver.ErrorResponse{}
	} else {
		logger.Info("unmount-volume-folder", lager.Data{"mountpath": mountPath})
		err := d.os.Remove(mountPath)
		if err != nil {
			logger.Error("unmount-failed", err)
			return voldriver.ErrorResponse{Err: fmt.Sprintf("Error unmounting volume: %s", err.Error())}
		}
	}

	logger.Info("unmounted-volume")

	d.volumes[name].Mountpoint = ""

	return voldriver.ErrorResponse{}
}
