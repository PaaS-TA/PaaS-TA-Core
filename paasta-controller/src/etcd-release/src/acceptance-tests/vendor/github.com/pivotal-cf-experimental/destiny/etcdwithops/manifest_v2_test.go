package etcdwithops_test

import (
	"io/ioutil"

	"github.com/pivotal-cf-experimental/destiny/etcdwithops"
	"github.com/pivotal-cf-experimental/gomegamatchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ManifestV2", func() {
	Describe("NewManifestV2", func() {
		Context("when ssl is enabled", func() {
			It("returns a YAML representation of the etcd manifest", func() {
				etcdManifest, err := ioutil.ReadFile("fixtures/etcd_manifest_v2_tls.yml")
				Expect(err).NotTo(HaveOccurred())

				manifest, err := etcdwithops.NewManifestV2(etcdwithops.ConfigV2{
					Name:      "some-manifest-name",
					AZs:       []string{"z1", "z2"},
					EnableSSL: true,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest).To(gomegamatchers.MatchYAML(etcdManifest))
			})
		})

		Context("when ssl is not enabled", func() {
			It("returns a YAML representation of the etcd manifest", func() {
				etcdManifest, err := ioutil.ReadFile("fixtures/etcd_manifest_v2_non_tls.yml")
				Expect(err).NotTo(HaveOccurred())

				manifest, err := etcdwithops.NewManifestV2(etcdwithops.ConfigV2{
					Name:      "some-manifest-name",
					AZs:       []string{"z1", "z2"},
					EnableSSL: false,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest).To(gomegamatchers.MatchYAML(etcdManifest))
			})
		})
	})
})
