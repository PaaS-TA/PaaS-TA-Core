package localdriver_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	"code.cloudfoundry.org/goshims/filepathshim/filepath_fake"
	"code.cloudfoundry.org/goshims/osshim/os_fake"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/localdriver"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Local Driver", func() {
	var (
		testLogger   lager.Logger
		ctx          context.Context
		env          voldriver.Env
		fakeOs       *os_fake.FakeOs
		fakeFilepath *filepath_fake.FakeFilepath
		localDriver  *localdriver.LocalDriver
		mountDir     string
		volumeId     string
	)
	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("localdriver-local")
		ctx = context.TODO()
		env = driverhttp.NewHttpDriverEnv(testLogger, ctx)

		mountDir = "/path/to/mount"

		fakeOs = &os_fake.FakeOs{}
		fakeFilepath = &filepath_fake.FakeFilepath{}
		localDriver = localdriver.NewLocalDriver(fakeOs, fakeFilepath, mountDir)
		volumeId = "test-volume-id"
	})

	Describe("#Activate", func() {
		It("returns Implements: VolumeDriver", func() {
			activateResponse := localDriver.Activate(env)
			Expect(len(activateResponse.Implements)).To(BeNumerically(">", 0))
			Expect(activateResponse.Implements[0]).To(Equal("VolumeDriver"))
		})
	})

	Describe("Mount", func() {

		Context("when the volume has been created", func() {
			BeforeEach(func() {
				createSuccessful(env, localDriver, fakeOs, volumeId)
				mountSuccessful(env, localDriver, volumeId, fakeFilepath)
			})

			AfterEach(func() {
				removeSuccessful(env, localDriver, volumeId)
			})

			Context("when the volume exists", func() {
				AfterEach(func() {
					unmountSuccessful(env, localDriver, volumeId)
				})

				It("should mount the volume on the local filesystem", func() {
					Expect(fakeFilepath.AbsCallCount()).To(Equal(3))
					Expect(fakeOs.MkdirAllCallCount()).To(Equal(4))
					Expect(fakeOs.SymlinkCallCount()).To(Equal(1))
					from, to := fakeOs.SymlinkArgsForCall(0)
					Expect(from).To(Equal("/path/to/mount/_volumes/test-volume-id"))
					Expect(to).To(Equal("/path/to/mount/_mounts/test-volume-id"))
				})

				It("returns the mount point on a /VolumeDriver.Get response", func() {
					getResponse := getSuccessful(env, localDriver, volumeId)
					Expect(getResponse.Volume.Mountpoint).To(Equal("/path/to/mount/_mounts/test-volume-id"))
				})
			})

			Context("when the volume is missing", func() {
				BeforeEach(func() {
					fakeOs.StatReturns(nil, os.ErrNotExist)
				})
				AfterEach(func() {
					fakeOs.StatReturns(nil, nil)
				})

				It("returns an error", func() {
					mountResponse := localDriver.Mount(env, voldriver.MountRequest{
						Name: volumeId,
					})
					Expect(mountResponse.Err).To(Equal("Volume 'test-volume-id' is missing"))
				})
			})
		})

		Context("when the volume has not been created", func() {
			It("returns an error", func() {
				mountResponse := localDriver.Mount(env, voldriver.MountRequest{
					Name: "bla",
				})
				Expect(mountResponse.Err).To(Equal("Volume 'bla' must be created before being mounted"))
			})
		})
	})

	Describe("Unmount", func() {
		Context("when a volume has been created", func() {
			BeforeEach(func() {
				createSuccessful(env, localDriver, fakeOs, volumeId)
			})

			Context("when a volume has been mounted", func() {
				BeforeEach(func() {
					mountSuccessful(env, localDriver, volumeId, fakeFilepath)
				})

				It("After unmounting /VolumeDriver.Get returns no mountpoint", func() {
					unmountSuccessful(env, localDriver, volumeId)
					getResponse := getSuccessful(env, localDriver, volumeId)
					Expect(getResponse.Volume.Mountpoint).To(Equal(""))
				})

				It("/VolumeDriver.Unmount doesn't remove mountpath from OS", func() {
					unmountSuccessful(env, localDriver, volumeId)
					Expect(fakeOs.RemoveCallCount()).To(Equal(1))
					removed := fakeOs.RemoveArgsForCall(0)
					Expect(removed).To(Equal("/path/to/mount/_mounts/test-volume-id"))
				})

				Context("when the same volume is mounted a second time then unmounted", func() {
					BeforeEach(func() {
						mountSuccessful(env, localDriver, volumeId, fakeFilepath)
						unmountSuccessful(env, localDriver, volumeId)
					})

					It("should not report empty mountpoint until unmount is called again", func() {
						getResponse := getSuccessful(env, localDriver, volumeId)
						Expect(getResponse.Volume.Mountpoint).NotTo(Equal(""))

						unmountSuccessful(env, localDriver, volumeId)
						getResponse = getSuccessful(env, localDriver, volumeId)
						Expect(getResponse.Volume.Mountpoint).To(Equal(""))
					})
				})
				Context("when the mountpath is not found on the filesystem", func() {
					var unmountResponse voldriver.ErrorResponse

					BeforeEach(func() {
						fakeOs.StatReturns(nil, os.ErrNotExist)
						unmountResponse = localDriver.Unmount(env, voldriver.UnmountRequest{
							Name: volumeId,
						})
					})

					It("returns an error", func() {
						Expect(unmountResponse.Err).To(Equal("Volume " + volumeId + " does not exist (path: /path/to/mount/_mounts/test-volume-id), nothing to do!"))
					})

					It("/VolumeDriver.Get still returns the mountpoint", func() {
						getResponse := getSuccessful(env, localDriver, volumeId)
						Expect(getResponse.Volume.Mountpoint).NotTo(Equal(""))
					})
				})

				Context("when the mountpath cannot be accessed", func() {
					var unmountResponse voldriver.ErrorResponse

					BeforeEach(func() {
						fakeOs.StatReturns(nil, errors.New("something weird"))
						unmountResponse = localDriver.Unmount(env, voldriver.UnmountRequest{
							Name: volumeId,
						})
					})

					It("returns an error", func() {
						Expect(unmountResponse.Err).To(Equal("Error establishing whether volume exists"))
					})

					It("/VolumeDriver.Get still returns the mountpoint", func() {
						getResponse := getSuccessful(env, localDriver, volumeId)
						Expect(getResponse.Volume.Mountpoint).NotTo(Equal(""))
					})
				})
			})

			Context("when the volume has not been mounted", func() {
				It("returns an error", func() {
					unmountResponse := localDriver.Unmount(env, voldriver.UnmountRequest{
						Name: volumeId,
					})

					Expect(unmountResponse.Err).To(Equal("Volume not previously mounted"))
				})
			})
		})

		Context("when the volume has not been created", func() {
			It("returns an error", func() {
				unmountResponse := localDriver.Unmount(env, voldriver.UnmountRequest{
					Name: volumeId,
				})

				Expect(unmountResponse.Err).To(Equal(fmt.Sprintf("Volume '%s' not found", volumeId)))
			})
		})
	})

	Describe("Create", func() {
		Context("when a second create is called with the same volume ID", func() {
			BeforeEach(func() {
				createSuccessful(env, localDriver, fakeOs, "volume")
			})

			Context("with the same opts", func() {
				It("does nothing", func() {
					createSuccessful(env, localDriver, fakeOs, "volume")
				})
			})
		})
	})

	Describe("Get", func() {
		Context("when the volume has been created", func() {
			It("returns the volume name", func() {
				createSuccessful(env, localDriver, fakeOs, volumeId)
				getSuccessful(env, localDriver, volumeId)
			})
		})

		Context("when the volume has not been created", func() {
			It("returns an error", func() {
				getUnsuccessful(env, localDriver, volumeId)
			})
		})
	})

	Describe("Path", func() {
		Context("when a volume is mounted", func() {
			BeforeEach(func() {
				createSuccessful(env, localDriver, fakeOs, volumeId)
				mountSuccessful(env, localDriver, volumeId, fakeFilepath)
			})

			It("returns the mount point on a /VolumeDriver.Path", func() {
				pathResponse := localDriver.Path(env, voldriver.PathRequest{
					Name: volumeId,
				})
				Expect(pathResponse.Err).To(Equal(""))
				Expect(pathResponse.Mountpoint).To(Equal("/path/to/mount/_mounts/" + volumeId))
			})
		})

		Context("when a volume is not created", func() {
			It("returns an error on /VolumeDriver.Path", func() {
				pathResponse := localDriver.Path(env, voldriver.PathRequest{
					Name: "volume-that-does-not-exist",
				})
				Expect(pathResponse.Err).NotTo(Equal(""))
				Expect(pathResponse.Mountpoint).To(Equal(""))
			})
		})

		Context("when a volume is created but not mounted", func() {
			var (
				volumeName string
			)
			BeforeEach(func() {
				volumeName = "my-volume"
				createSuccessful(env, localDriver, fakeOs, volumeName)
			})

			It("returns an error on /VolumeDriver.Path", func() {
				pathResponse := localDriver.Path(env, voldriver.PathRequest{
					Name: "volume-that-does-not-exist",
				})
				Expect(pathResponse.Err).NotTo(Equal(""))
				Expect(pathResponse.Mountpoint).To(Equal(""))
			})
		})
	})

	Describe("List", func() {
		Context("when there are volumes", func() {
			BeforeEach(func() {
				createSuccessful(env, localDriver, fakeOs, volumeId)
			})

			It("returns the list of volumes", func() {
				listResponse := localDriver.List(env)

				Expect(listResponse.Err).To(Equal(""))
				Expect(listResponse.Volumes[0].Name).To(Equal(volumeId))

			})
		})

		Context("when the volume has not been created", func() {
			It("returns an error", func() {
				volumeName := "test-volume-2"
				getUnsuccessful(env, localDriver, volumeName)
			})
		})
	})

	Describe("Remove", func() {
		It("should fail if no volume name provided", func() {
			removeResponse := localDriver.Remove(env, voldriver.RemoveRequest{
				Name: "",
			})
			Expect(removeResponse.Err).To(Equal("Missing mandatory 'volume_name'"))
		})

		It("should fail if no volume was created", func() {
			removeResponse := localDriver.Remove(env, voldriver.RemoveRequest{
				Name: volumeId,
			})
			Expect(removeResponse.Err).To(Equal("Volume '" + volumeId + "' not found"))
		})

		Context("when the volume has been created", func() {
			BeforeEach(func() {
				createSuccessful(env, localDriver, fakeOs, volumeId)
			})

			It("/VolumePlugin.Remove destroys volume", func() {
				removeResponse := localDriver.Remove(env, voldriver.RemoveRequest{
					Name: volumeId,
				})
				Expect(removeResponse.Err).To(Equal(""))
				Expect(fakeOs.RemoveAllCallCount()).To(Equal(1))

				getUnsuccessful(env, localDriver, volumeId)
			})

			Context("when volume has been mounted", func() {
				It("/VolumePlugin.Remove unmounts and destroys volume", func() {
					mountSuccessful(env, localDriver, volumeId, fakeFilepath)

					removeResponse := localDriver.Remove(env, voldriver.RemoveRequest{
						Name: volumeId,
					})
					Expect(removeResponse.Err).To(Equal(""))
					Expect(fakeOs.RemoveCallCount()).To(Equal(1))
					Expect(fakeOs.RemoveAllCallCount()).To(Equal(1))

					getUnsuccessful(env, localDriver, volumeId)
				})
			})
		})

		Context("when the volume has not been created", func() {
			It("returns an error", func() {
				removeResponse := localDriver.Remove(env, voldriver.RemoveRequest{
					Name: volumeId,
				})
				Expect(removeResponse.Err).To(Equal("Volume '" + volumeId + "' not found"))
			})
		})
	})
})

func getUnsuccessful(env voldriver.Env, localDriver voldriver.Driver, volumeName string) {
	getResponse := localDriver.Get(env, voldriver.GetRequest{
		Name: volumeName,
	})

	Expect(getResponse.Err).To(Equal("Volume not found"))
	Expect(getResponse.Volume.Name).To(Equal(""))
}

func getSuccessful(env voldriver.Env, localDriver voldriver.Driver, volumeName string) voldriver.GetResponse {
	getResponse := localDriver.Get(env, voldriver.GetRequest{
		Name: volumeName,
	})

	Expect(getResponse.Err).To(Equal(""))
	Expect(getResponse.Volume.Name).To(Equal(volumeName))
	return getResponse
}

func createSuccessful(env voldriver.Env, localDriver voldriver.Driver, fakeOs *os_fake.FakeOs, volumeName string) {
	createResponse := localDriver.Create(env, voldriver.CreateRequest{
		Name: volumeName,
	})
	Expect(createResponse.Err).To(Equal(""))

	Expect(fakeOs.MkdirAllCallCount()).Should(Equal(2))

	volumeDir, fileMode := fakeOs.MkdirAllArgsForCall(1)
	Expect(path.Base(volumeDir)).To(Equal(volumeName))
	Expect(fileMode).To(Equal(os.ModePerm))
}

func mountSuccessful(env voldriver.Env, localDriver voldriver.Driver, volumeName string, fakeFilepath *filepath_fake.FakeFilepath) {
	fakeFilepath.AbsReturns("/path/to/mount/", nil)
	mountResponse := localDriver.Mount(env, voldriver.MountRequest{
		Name: volumeName,
	})
	Expect(mountResponse.Err).To(Equal(""))
	Expect(mountResponse.Mountpoint).To(Equal("/path/to/mount/_mounts/" + volumeName))
}

func unmountSuccessful(env voldriver.Env, localDriver voldriver.Driver, volumeName string) {
	unmountResponse := localDriver.Unmount(env, voldriver.UnmountRequest{
		Name: volumeName,
	})
	Expect(unmountResponse.Err).To(Equal(""))
}

func removeSuccessful(env voldriver.Env, localDriver voldriver.Driver, volumeName string) {
	removeResponse := localDriver.Remove(env, voldriver.RemoveRequest{
		Name: volumeName,
	})
	Expect(removeResponse.Err).To(Equal(""))
}
