package etcdstoreadapter_test

import (
	"flag"

	. "github.com/cloudfoundry/storeadapter/etcdstoreadapter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Flags", func() {
	Describe("Validate", func() {
		var sslFlagSet, httpFlagSet *flag.FlagSet
		var sslFlags, httpFlags *ETCDFlags
		var sslCommandLine, httpCommandLine []string

		BeforeEach(func() {
			sslFlagSet = flag.NewFlagSet("ssl", flag.ExitOnError)
			sslFlags = AddFlags(sslFlagSet)
			httpFlagSet = flag.NewFlagSet("http", flag.ExitOnError)
			httpFlags = AddFlags(httpFlagSet)
			sslCommandLine = []string{
				"-etcdCertFile", "../assets/client.crt",
				"-etcdKeyFile", "../assets/client.key",
				"-etcdCaFile", "../assets/ca.crt",
				"-etcdCluster", "https://mycluster1, https://mycluster2:435",
			}
			httpCommandLine = []string{
				"-etcdCluster", "http://a.b.c, http://d.e.f",
			}
		})

		JustBeforeEach(func() {
			err := sslFlagSet.Parse(sslCommandLine)
			Expect(err).NotTo(HaveOccurred())
			err = httpFlagSet.Parse(httpCommandLine)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when a url cannot be parsed", func() {
			BeforeEach(func() {
				httpCommandLine = []string{
					"-etcdCluster", "http://foo, http://%",
				}
			})

			It("should fail", func() {
				_, err := httpFlags.Validate()
				Expect(err).To(MatchError(HavePrefix("Invalid cluster URL:")))
			})
		})

		Context("when the scheme is not http/https", func() {
			BeforeEach(func() {
				httpCommandLine = []string{
					"-etcdCluster", "htt://foo",
				}
			})

			It("should fail", func() {
				_, err := httpFlags.Validate()
				Expect(err).To(MatchError(HavePrefix("Invalid scheme:")))
			})
		})

		Context("when urls have mixed schemes", func() {
			BeforeEach(func() {
				httpCommandLine = []string{
					"-etcdCluster", "http://foo, https://bar",
				}
			})

			It("should fail", func() {
				_, err := httpFlags.Validate()
				Expect(err).To(MatchError("Multiple url schemes provided: http://foo, https://bar"))
			})
		})

		Context("When the ETCD cluster URL is https", func() {
			It("should succeed", func() {
				options, err := sslFlags.Validate()
				Expect(err).NotTo(HaveOccurred())
				Expect(options).To(Equal(&ETCDOptions{
					CertFile:    "../assets/client.crt",
					KeyFile:     "../assets/client.key",
					CAFile:      "../assets/ca.crt",
					ClusterUrls: []string{"https://mycluster1", "https://mycluster2:435"},
					IsSSL:       true,
				}))
			})

			Context("when a cert file is not provided", func() {
				BeforeEach(func() {
					sslCommandLine = append(sslCommandLine, "-etcdCertFile", "")
				})

				It("should fail", func() {
					_, err := sslFlags.Validate()
					Expect(err).To(MatchError("Cert file must be provided for https connections"))
				})
			})

			Context("when a key file is not provided", func() {
				BeforeEach(func() {
					sslCommandLine = append(sslCommandLine, "-etcdKeyFile", "")
				})

				It("should fail", func() {
					_, err := sslFlags.Validate()
					Expect(err).To(MatchError("Key file must be provided for https connections"))
				})
			})

			Context("when a CA cert file is not provided", func() {
				BeforeEach(func() {
					sslCommandLine = append(sslCommandLine, "-etcdCaFile", "")
				})

				It("should succeed", func() {
					// A CA cert file is not technically needed because the needed CA cert
					// may already be in the system.
					_, err := sslFlags.Validate()
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("When the ETCD cluster URL is http", func() {
			It("should succeed", func() {
				options, err := httpFlags.Validate()
				Expect(err).NotTo(HaveOccurred())
				Expect(options).To(Equal(&ETCDOptions{
					CertFile:    "",
					KeyFile:     "",
					CAFile:      "",
					ClusterUrls: []string{"http://a.b.c", "http://d.e.f"},
					IsSSL:       false,
				}))
			})

			Context("when ssl configuration is provided", func() {
				BeforeEach(func() {
					httpCommandLine = append(httpCommandLine,
						"-etcdCertFile", "cert",
						"-etcdKeyFile", "key",
						"-etcdCaFile", "ca",
					)
				})

				It("should pass", func() {
					options, err := httpFlags.Validate()
					Expect(err).NotTo(HaveOccurred())
					Expect(options.IsSSL).To(BeFalse())
				})
			})
		})
	})
})
