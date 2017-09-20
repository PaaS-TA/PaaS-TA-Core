package consul_test

import (
	"github.com/pivotal-cf-experimental/destiny/consul"
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Job", func() {
	Context("Manifest V2", func() {
		var (
			manifest consul.ManifestV2
		)
		BeforeEach(func() {
			var err error
			manifest, err = consul.NewManifestV2(consul.ConfigV2{
				AZs: []consul.ConfigAZ{
					{
						IPRange: "10.244.4.0/24",
						Nodes:   1,
						Name:    "z1",
					},
				},
			}, iaas.NewWardenConfig())
			Expect(err).NotTo(HaveOccurred())
		})
		Describe("SetInstanceCount", func() {
			It("updates the instance count", func() {
				newManifest, err := manifest.SetInstanceCount("consul", 3)
				Expect(err).NotTo(HaveOccurred())

				consulInstanceGroup, err := newManifest.GetInstanceGroup("consul")
				Expect(err).NotTo(HaveOccurred())

				Expect(consulInstanceGroup.Instances).To(Equal(3))
				Expect(len(consulInstanceGroup.Networks[0].StaticIPs)).To(Equal(3))
			})

			It("updates static ips to empty slice if instance count is 0", func() {
				newManifest, err := manifest.SetInstanceCount("consul", 0)
				Expect(err).NotTo(HaveOccurred())

				consulInstanceGroup, err := newManifest.GetInstanceGroup("consul")
				Expect(err).NotTo(HaveOccurred())

				Expect(consulInstanceGroup.Instances).To(Equal(0))
				Expect(consulInstanceGroup.Networks[0].StaticIPs).To(Equal([]string{}))
			})
		})

		Describe("GetInstanceGroup", func() {
			It("gets the instance group from the manifest", func() {
				consulInstanceGroup, err := manifest.GetInstanceGroup("consul")
				Expect(err).NotTo(HaveOccurred())
				Expect(consulInstanceGroup.Name).To(Equal("consul"))
			})

			Context("failure cases", func() {
				It("errors out when it cannot find the instance group", func() {
					_, err := manifest.GetInstanceGroup("non-existing-job")
					Expect(err).To(MatchError(`instance group "non-existing-job" does not exist`))
				})
			})
		})
	})

	Context("Manifest V1", func() {
		Describe("SetConsulJobInstanceCount", func() {
			var (
				manifest consul.Manifest
			)

			BeforeEach(func() {
				var err error
				manifest, err = consul.NewManifest(consul.Config{
					Networks: []consul.ConfigNetwork{
						{
							IPRange: "10.244.4.0/24",
							Nodes:   1,
						},
					},
				}, iaas.NewWardenConfig())
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets the correct values for instances and static_ips given a count", func() {
				job := manifest.Jobs[0]
				properties := manifest.Properties

				Expect(job.Instances).To(Equal(1))
				Expect(job.Networks[0].StaticIPs).To(HaveLen(1))
				Expect(job.Networks[0].StaticIPs).To(Equal([]string{"10.244.4.4"}))
				Expect(properties.Consul.Agent.Servers.Lan).To(Equal([]string{"10.244.4.4"}))

				manifest, err := manifest.SetConsulJobInstanceCount(3)
				Expect(err).NotTo(HaveOccurred())

				job = manifest.Jobs[0]
				properties = manifest.Properties

				Expect(job.Instances).To(Equal(3))
				Expect(job.Networks[0].StaticIPs).To(HaveLen(3))
				Expect(job.Networks[0].StaticIPs).To(Equal([]string{"10.244.4.4", "10.244.4.5", "10.244.4.6"}))
				Expect(properties.Consul.Agent.Servers.Lan).To(Equal([]string{"10.244.4.4", "10.244.4.5", "10.244.4.6"}))
			})

			Context("failure cases", func() {
				It("returns an error when set job instance count fails", func() {
					manifest.Jobs[0].Networks = []core.JobNetwork{}
					_, err := manifest.SetConsulJobInstanceCount(3)
					Expect(err).To(MatchError(`"consul_z1" job must have an existing network to modify`))
				})
			})
		})

		Describe("SetJobInstanceCount", func() {
			var (
				manifest consul.Manifest
			)

			BeforeEach(func() {
				var err error
				manifest, err = consul.NewManifest(consul.Config{
					Networks: []consul.ConfigNetwork{
						{
							IPRange: "10.244.4.0/24",
							Nodes:   1,
						},
					},
				}, iaas.NewWardenConfig())
				Expect(err).NotTo(HaveOccurred())

			})

			It("sets the correct values for instances and static_ips given a count", func() {
				var err error
				job := findJob(manifest, "consul_test_consumer")
				network := manifest.Networks[0]

				Expect(job.Instances).To(Equal(1))
				Expect(job.Networks[0].StaticIPs).To(HaveLen(1))
				Expect(job.Networks[0].Name).To(Equal(network.Name))
				Expect(job.Networks[0].StaticIPs).To(Equal([]string{"10.244.4.10"}))

				manifest, err = manifest.SetJobInstanceCount("consul_test_consumer", 3)
				Expect(err).NotTo(HaveOccurred())

				job = findJob(manifest, "consul_test_consumer")

				Expect(job.Instances).To(Equal(3))
				Expect(job.Networks[0].StaticIPs).To(HaveLen(3))
				Expect(job.Networks[0].Name).To(Equal(network.Name))
				Expect(job.Networks[0].StaticIPs).To(Equal([]string{"10.244.4.10", "10.244.4.11", "10.244.4.12"}))
			})

			It("sets the correct values given a count of zero", func() {
				var err error
				manifest, err = manifest.SetJobInstanceCount("consul_test_consumer", 0)
				Expect(err).NotTo(HaveOccurred())

				job := findJob(manifest, "consul_test_consumer")
				Expect(job.Instances).To(Equal(0))
				Expect(job.Networks[0].StaticIPs).To(HaveLen(0))
			})

			Context("failure cases", func() {
				It("returns an error when the job does not exist", func() {
					_, err := manifest.SetJobInstanceCount("fake-job", 3)
					Expect(err).To(MatchError(`"fake-job" job does not exist`))
				})

				It("returns an error when the networks is empty", func() {
					manifest.Jobs[1].Networks = []core.JobNetwork{}
					_, err := manifest.SetJobInstanceCount("consul_test_consumer", 3)
					Expect(err).To(MatchError(`"consul_test_consumer" job must have an existing network to modify`))
				})

				It("returns an error when the static ips is empty", func() {
					manifest.Jobs[1].Networks[0].StaticIPs = []string{}
					_, err := manifest.SetJobInstanceCount("consul_test_consumer", 3)
					Expect(err).To(MatchError(`"consul_test_consumer" job must have at least one static ip in its network`))
				})

				It("returns an error when the job does not exist", func() {
					manifest.Jobs[1].Networks[0].StaticIPs[0] = "%%%%"
					_, err := manifest.SetJobInstanceCount("consul_test_consumer", 3)
					Expect(err).To(MatchError(`'%%%%' is not a valid ip address`))
				})
			})
		})
	})
})
