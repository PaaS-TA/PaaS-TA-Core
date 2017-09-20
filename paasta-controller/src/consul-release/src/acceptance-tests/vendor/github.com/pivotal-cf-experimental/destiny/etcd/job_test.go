package etcd_test

import (
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/etcd"
	"github.com/pivotal-cf-experimental/destiny/iaas"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manifest", func() {
	Describe("Job", func() {
		Describe("SetEtcdProperties", func() {
			It("updates the etcd and testconsumer properties to match the current job configuration", func() {
				manifest, err := etcd.NewManifest(etcd.Config{
					IPRange: "10.244.4.0/24",
				}, iaas.NewWardenConfig())
				Expect(err).NotTo(HaveOccurred())

				job := manifest.Jobs[0]
				properties := manifest.Properties

				Expect(properties.EtcdTestConsumer.Etcd.Machines).To(Equal(job.Networks[0].StaticIPs))
				Expect(properties.Etcd.Machines).To(Equal(job.Networks[0].StaticIPs))
				Expect(properties.Etcd.Cluster[0].Instances).To(Equal(1))

				job.Instances = 3
				job.Networks[0].StaticIPs = []string{"ip1", "ip2", "ip3"}

				properties = etcd.SetEtcdProperties(job, properties)
				Expect(properties.EtcdTestConsumer.Etcd.Machines).To(Equal(job.Networks[0].StaticIPs))
				Expect(properties.Etcd.Machines).To(Equal(job.Networks[0].StaticIPs))
				Expect(properties.Etcd.Cluster[0].Instances).To(Equal(3))
			})

			It("does not override the machines property if ssl is enabled", func() {
				manifest, err := etcd.NewTLSManifest(etcd.Config{
					IPRange: "10.244.4.0/24",
				}, iaas.NewWardenConfig())
				Expect(err).NotTo(HaveOccurred())

				job := manifest.Jobs[0]
				properties := manifest.Properties

				Expect(properties.EtcdTestConsumer.Etcd.Machines).To(Equal([]string{"etcd.service.cf.internal"}))
				Expect(properties.Etcd.Machines).To(Equal([]string{"etcd.service.cf.internal"}))
				Expect(properties.Etcd.Cluster[0].Instances).To(Equal(1))

				job.Instances = 3
				job.Networks[0].StaticIPs = []string{"ip1", "ip2", "ip3"}

				properties = etcd.SetEtcdProperties(job, properties)
				Expect(properties.EtcdTestConsumer.Etcd.Machines).To(Equal([]string{"etcd.service.cf.internal"}))
				Expect(properties.Etcd.Machines).To(Equal([]string{"etcd.service.cf.internal"}))
				Expect(properties.Etcd.Cluster[0].Instances).To(Equal(3))
			})
		})

		Describe("SetJobInstanceCount", func() {
			var (
				manifest etcd.Manifest
			)

			BeforeEach(func() {
				var err error
				manifest, err = etcd.NewManifest(etcd.Config{
					IPRange: "10.244.4.0/24",
				}, iaas.NewWardenConfig())
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets the correct values for instances and static_ips given a count", func() {
				var err error
				job := findJob(manifest, "etcd_z1")
				network := manifest.Networks[0]

				Expect(job.Instances).To(Equal(1))
				Expect(job.Networks[0].StaticIPs).To(HaveLen(1))
				Expect(job.Networks[0].Name).To(Equal(network.Name))
				Expect(job.Networks[0].StaticIPs).To(Equal([]string{"10.244.4.4"}))

				manifest, err = manifest.SetJobInstanceCount("etcd_z1", 3)
				Expect(err).NotTo(HaveOccurred())

				job = findJob(manifest, "etcd_z1")

				Expect(job.Instances).To(Equal(3))
				Expect(job.Networks[0].StaticIPs).To(HaveLen(3))
				Expect(job.Networks[0].Name).To(Equal(network.Name))
				Expect(job.Networks[0].StaticIPs).To(Equal([]string{"10.244.4.4", "10.244.4.5", "10.244.4.6"}))
			})

			It("sets the correct values given a count of zero", func() {
				var err error
				manifest, err = manifest.SetJobInstanceCount("etcd_z1", 0)
				Expect(err).NotTo(HaveOccurred())

				job := findJob(manifest, "etcd_z1")
				Expect(job.Instances).To(Equal(0))
				Expect(job.Networks[0].StaticIPs).To(HaveLen(0))
			})

			Context("failure cases", func() {
				It("returns an error when the job does not exist", func() {
					_, err := manifest.SetJobInstanceCount("fake-job", 3)
					Expect(err).To(MatchError(`"fake-job" job does not exist`))
				})

				It("returns an error when the networks is empty", func() {
					manifest.Jobs[0].Networks = []core.JobNetwork{}
					_, err := manifest.SetJobInstanceCount("etcd_z1", 3)
					Expect(err).To(MatchError(`"etcd_z1" job must have an existing network to modify`))
				})

				It("returns an error when the static ips is empty", func() {
					manifest.Jobs[0].Networks[0].StaticIPs = []string{}
					_, err := manifest.SetJobInstanceCount("etcd_z1", 3)
					Expect(err).To(MatchError(`"etcd_z1" job must have at least one static ip in its network`))
				})

				It("returns an error when the job does not exist", func() {
					manifest.Jobs[0].Networks[0].StaticIPs[0] = "%%%%"
					_, err := manifest.SetJobInstanceCount("etcd_z1", 3)
					Expect(err).To(MatchError(`'%%%%' is not a valid ip address`))
				})
			})
		})
	})

	Describe("RemoveJob", func() {
		It("returns a manifest with specified job removed", func() {
			manifest := etcd.Manifest{
				Jobs: []core.Job{
					{Name: "some-job-name-1"},
					{Name: "some-job-name-2"},
					{Name: "some-job-name-3"},
				},
			}

			manifest = manifest.RemoveJob("some-job-name-2")

			Expect(manifest.Jobs).To(Equal([]core.Job{
				{Name: "some-job-name-1"},
				{Name: "some-job-name-3"},
			}))
		})
	})
})
