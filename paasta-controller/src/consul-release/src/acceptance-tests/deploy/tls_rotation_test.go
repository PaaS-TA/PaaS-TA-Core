package deploy_test

import (
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/consulclient"
	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	newCACert = `-----BEGIN CERTIFICATE-----
MIIE5jCCAs6gAwIBAgIBATANBgkqhkiG9w0BAQsFADATMREwDwYDVQQDEwhjb25z
dWxDQTAeFw0xNzAxMTYyMzEwNDlaFw0yNzAxMTYyMzEwNTFaMBMxETAPBgNVBAMT
CGNvbnN1bENBMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAseVoqSfG
5B4rhVj4OaXDPnRLFdicYQVr4dmrI+kuq61T4/yT+D6nCTkemizeHS1qYWoW0ukl
YsEHNc+QRPdrN0aRmvPUjrqPF6Pg85ssb3l0jOBUS8mzwlei1e+rNlJ9seoCRnN4
/GuqZtkXVdHEQF83/DDOy6f+diTmFL9JMn94DrM3ZR4xlk1+llMIdLZhBReh3Qp/
4dJCBAKNcAyfCitlARo78hsHsNyFigD9Heyv9KdjOoJrnzJwTtUZLmTlVSIZy1ax
0WpTU5z3J6loozSNOQuMKa5+EWSmRojBiO2Hw8ttN2oUPU9HBE+2+ArnuafI+yxq
IB9gU/6STFX4QHWgexPgWCNn1e2yCqXIKTF775DvuZBZT59xSr+SrGCGXOGj9wlZ
M0+FskoVzFivox3Pq43CkgQgXOhxWFWmrJCp6MphY225ahn8hi+a+P0ijUiP0veY
Ih/I4+R6ffKGlp4sSn6Soz79zw1kt0FvmiPJcwymq2BRfnOe6gyGSXfjYG2ijDgU
BTyPtkI10xe9y3Xyx4VC6T2aqfnkZ+5wZ6JcX08cXGbahxIYFMncnVRn4OOntX7A
Z4XB3I+tdcStQnrGOlGnP9rsDayOEfBVdRK1jyar1lqZuV+mwoCIsIcZhcAkArmV
Zk9sRYspsiEGF6mb4R6YW8+VK1wFTE0LCO8CAwEAAaNFMEMwDgYDVR0PAQH/BAQD
AgEGMBIGA1UdEwEB/wQIMAYBAf8CAQAwHQYDVR0OBBYEFJd5aM+4LDKjByJrnrGL
1NLFtEszMA0GCSqGSIb3DQEBCwUAA4ICAQAV+1wrYVXTnQdkilhY+6YZlAJHgFmH
6KFOHTHCmy4jOlOZyVzkWEJUsw+NuG/BGOv/66bkzLczxo/R/NDkoSnYSKya5J4/
9WO18fzoS9Qe368ZS0aET/VWeItAiV2gGbpK51OSIFxpK+O0grbd4wh/TChoU7VU
qbCScoxw/AAY/+sE1AE3Yz8fmRFCWX5Yzn1mfH+xoyxheQ9zGCyccW1cjemO2OVz
+laBrfnY2wo4CPKs7uavybCyEuBje14EcaJmt7zZ0BPwTETRvLjx817HsaxdXSzD
U2lNDr5GlCEGbzj5kguwxRgKtnyXjEOkZ3e7od+FE+14dilrAy0R6fbnzQAr+YNA
zm133WhlT577GIDff+7uMLDlfE/7XChOPrEjCQjgI3inFUWlQOp/+1d/xw/wq4Ps
l4cZYD8i3sAtOa+u4k0YHEKaUhd/HnAD2hNF5gVaMKfQbV+wBCNWhccD2Luy6Jkb
2zQHv5cCZ8izd+3dOs81xNPIB489fKjh3COQtzy9yP1SFxQsZ0cm/pjh6sxFvjSI
Gzs3p8kxbTTB3O/l01rgVj9EMcPqoxsaA5qMObmYp+mbyEeJ94jEh1h+u3qdwAml
CUKikmFIsN2gXxI3PhR+mSnagGAATmyyOTDSD0lbl0cvqku/tYxVa483NWA4gzB6
SR8rzYdeD2d0YQ==
-----END CERTIFICATE-----`
	newAgentCert = `-----BEGIN CERTIFICATE-----
MIIEOTCCAiGgAwIBAgIRANC1J6I0SiVktHFi6Vim2A4wDQYJKoZIhvcNAQELBQAw
EzERMA8GA1UEAxMIY29uc3VsQ0EwHhcNMTcwMTE2MjMxMDUyWhcNMTkwMTE2MjMx
MDUyWjAXMRUwEwYDVQQDEwxjb25zdWwgYWdlbnQwggEiMA0GCSqGSIb3DQEBAQUA
A4IBDwAwggEKAoIBAQCesalBmSbx/23SncA0kNX6wOYKLP3uQFFSGFSXdLdsAKHC
t6FH89dbDzExCpNTJNgsKUcsiw1vIXhaGQl+GmSeXVf0m7ha5aXXH0xeRAH4AvUo
xTZjM9QftV8E4IE71tgLx5NWCo8YIfz8XoWd/PSgLZ91wzolf6u3zSJs4Arl/55P
pK2NRmcpUL42tUkGd+I0N6vwl1gyhcHfeyOpSz1JfeWwX3IEN7cpw1JiQ8PprwC6
KgX6LjAyxCjMMk1UOktPcEHsVPR/6ezPG3w/tt0GU57WyVo3giUJhB3pmEzpGP8K
8qbJZVeu6BzZbbwd4Tpaaxt+82pxRni2BVNNkba5AgMBAAGjgYMwgYAwDgYDVR0P
AQH/BAQDAgO4MB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAdBgNVHQ4E
FgQU/pvyNw6DfhusyL3Gwa4n5+JvSoYwHwYDVR0jBBgwFoAUl3loz7gsMqMHImue
sYvU0sW0SzMwDwYDVR0RBAgwBocEfwAAATANBgkqhkiG9w0BAQsFAAOCAgEAWQC7
fyLOTXC+Z1NpCsiIFgSlSAL0gFbGippprRzPRM7cxDsAjuNLWsfmUtLi8VpCAiEX
No4ILjodLCywLI0vyCJU8qneH9UGYqDMx5kR8r9Iqyh13x80GZAYZjHLYxS8O+FT
ql3XIN3IsoZlFRDYn0SSnv44neEnfnoMwaDNjxSkxZJ6kiP1UUyyW8NU9EbZq2Q0
h5fbj7GcQ0zBltRiE8sxj0ZnA6d4Nlfcdjh/oX7csHzyqsoAFzPXri7E4j2L91g1
Kc4MESayzlB4aPNBTd6FaiczxTKto5sdWwiBxOoyqMfVYhO+oJh/rZpM3HhwMvdw
F/gEHl1uLxHM7clSds+krdDCxRytIwOhm4PJke6oeVFTsuL8hSi9GIFgFRnx9nNg
ByaoIHNnLiryU8RHGygjXHHL2xFSKH2uWJPBHE/zulaFosmNFp775lYpsD3RG89n
bJ9LBnw8EDkrInJ6t3xlVBEFJnNaihZMn1kMdjAxjvOCUyHsprAIbiF5tKUUmdCI
0LXFbQEIgUIKcWhowwCtuBcPJiJTbfYlQjjrwO6qb6pq3YAo523NRBWm1dSxHkRj
wRs/fWmuu/Olxo34mFzz1gKf/br3d1DBTg7cvLbQP4Zea+PSJKUrjG2kPIHe6RY6
D8hDL8PfJ16wNv9LllCRyJJIZ3or6L3tsn4x1QA=
-----END CERTIFICATE-----`
	newAgentKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAnrGpQZkm8f9t0p3ANJDV+sDmCiz97kBRUhhUl3S3bAChwreh
R/PXWw8xMQqTUyTYLClHLIsNbyF4WhkJfhpknl1X9Ju4WuWl1x9MXkQB+AL1KMU2
YzPUH7VfBOCBO9bYC8eTVgqPGCH8/F6Fnfz0oC2fdcM6JX+rt80ibOAK5f+eT6St
jUZnKVC+NrVJBnfiNDer8JdYMoXB33sjqUs9SX3lsF9yBDe3KcNSYkPD6a8AuioF
+i4wMsQozDJNVDpLT3BB7FT0f+nszxt8P7bdBlOe1slaN4IlCYQd6ZhM6Rj/CvKm
yWVXrugc2W28HeE6WmsbfvNqcUZ4tgVTTZG2uQIDAQABAoIBAA7napkBlDnIHn1Y
WXPWYnJRaYltHlAg9EI8jL1Ite1LxeVur5P9X61qqNkNQDbfz/mdytRxHsrgHth/
X3fbbLW+2ILdmRvYU5H3m4mC45hyVqoEk44PkQ2FUC46E4kWLWY10S2Ugknm70aY
bf4fgq4EeuRpeG2LJwp1FpWZGQzupgcPx+Y2e13QHE9fR17Ot0L5726pEC1ePUfh
ZbQ0XubGvK3h4IpUO5zIpdavZt84+vBZcSX/fhsxXlyzcfE6K+pVQA302AQYMp5q
TogQ62j4i0F6TgBq3bkMvpIgYYH+3FdJq491v/s6VF2olJW8G+BMvNMXfDvX46Nc
p+TIjc0CgYEAzRL4y6YCNC6g43MwwGG7QbCMQLuDJFfz5+5fWLVlbS3nx1SJGSay
ZhgGv5fFp8UcMvF3uVfOkJxIF7sQmxtv+IECkVAq8bBYxHoAxUg2JC/K66Qm388L
8UOFOSluajn6ql4jjq7l/VWBJqbNJE3Pel/0vjkgsqojfLIFo1M6y5sCgYEAxhoz
Dwf7zfsqi+KFYFZKVCnycPbA0SM2x22mJU67YmxKUZMXFq79ECSmdklWvZPsAyy1
uxW5QBBJ14AbjizOmf3A5laq8qz0E2LVN2NVJ7o2K/BlhpCFQLZLxY/V4ms5hJ/q
N/UtJfxOUH6zodLJGsHUcNQDkz8Kt1Yhkw2T/jsCgYAZ3L+lpyz1+b9uj9NhH7Im
6aX2b+9tAO6QnF5H6LB+4WAuojmcA2ZSO8t2FCToMJKK1ir8I9e4Iw1weLXyabZo
R5TUUKDp1AyN0rkQKDgzvhdAOnZwmULvTU2a1N/I48D6BV9EmkgE9+iOwFB0uJ9m
1n0eFERMY+qPyj+txkxO6wKBgC76BYOZ/A9TcTpsw/4dWFDvBYveJ8kwVYwjJ1QP
gIYNce44ODBr3JzYZBUGvSgFjOEP2CR+OUjE1A3jViV34KJJt1Wn1a/obZSvSipx
Rr89/BydTCYF3WOEFyHJQwoHLUOS/GK6pDMuyo9yDDzilEfhEPSUgiiHuY3SQfHy
NVcDAoGADgyMBOr5HBRC+h4hHW68KYntuH6u+eryfmvTmGgvDLnbzxB7u9x+k4mc
7JPngQnezlSfw/oyPFkb6i2FQtIDjoChOldVCiZe/vqXKN+oXtWKAU1qOBsDOUMq
XjO9kda42lqcXJKy2KaMc5zx0eMGRNklWUgYbqQ5MFSNobN0/3E=
-----END RSA PRIVATE KEY-----`
	newServerCert = `-----BEGIN CERTIFICATE-----
MIIEMDCCAhigAwIBAgIRAIb6XAn6XTJPJ4arqIVGkUswDQYJKoZIhvcNAQELBQAw
EzERMA8GA1UEAxMIY29uc3VsQ0EwHhcNMTcwMTE2MjMxMDUyWhcNMTkwMTE2MjMx
MDUyWjAhMR8wHQYDVQQDExZzZXJ2ZXIuZGMxLmNmLmludGVybmFsMIIBIjANBgkq
hkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0XRZI3u7vNwoPrZwiJerfTVKUC1Jz4dX
52rW+9ivChhL/whs5lzBOhSeISUcKfpIwQwLXh49A7zgetA9GM6wfFO+u+rcMUP9
H4uaZT4mlZu/7cjXs6ZLIbIyKNLMuGVoOT8a3KNnPau3nB1sGUl8LPjyXO+scHGd
aw1K/hIbCbceXHoNZz7DrgWQtIJByn80ePcjiMrRqovKQCQdZCU6KUafc/rvrrY9
juWeu8MYGIvDuTGFW11M4uB2sHvDTI6fokowE/gcnq5m7+C4jCHvFKfMWG9xtOCx
LdxsFRU0UmVP8uKCDYVuHG0GNh2xYYjifpL0GGBdbUzn2OgQkDFNcwIDAQABo3Ew
bzAOBgNVHQ8BAf8EBAMCA7gwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMC
MB0GA1UdDgQWBBQP8tFaArPSNrTxWXCIhhsUUOFd4DAfBgNVHSMEGDAWgBSXeWjP
uCwyowcia56xi9TSxbRLMzANBgkqhkiG9w0BAQsFAAOCAgEAUVl1x+hkBjo7iU96
smYd1DW4HFP4cnLmP1awI/HwWpk7jQUlHtyARquhohKWfMYWe6b3EnjTs4Tub2hq
4XuWRvzy0geyy3ATwwB2facZSdAMqsRFsAnjlECippeOF9HsMKoLZjYx/vuRiOCo
mbPCO+erH6OHTysacO2EdLrGdAXLok8uwV20a4OheLBiXybVA6/fIZPHPV6+yRmQ
+M+ukYu8Bzo6pCPmuP6XDUGHWuWqUEUxT/3z0dOLDeoAdAnjPFxt4V8JVTtzUcnh
WF6+LOVL+9ZVvKsNjL4b1SBq5dRxFvOsurOVRWHW8s5VbFFyAVvFcX17fypKvlGP
UKtVzD4wajagoTTt3u7Iw02KwAN1Ynz7EtEytXBibmkK/E9IKfM014Tgos0f4LsB
8uO0MNCsf0MBkCBxy2/+8G6pOsaReVm/I5RlNg1kUo0rwwseXaoMQXlZ1oOsF9f4
fUSm1kIzz61SOrjy+yFjzfymaRwTKiCP86ojhWxDdIWs9J14BN3Hi7RNpBcVALsA
Hg1lJM2DzWOvnmclSg4k9wsrM7wj6Fd/7T6sskUnn9NNO28tiXBGTv1YJ0DpJEuU
pxFM82SHv8vMJLB0SLYDN5VRvqrlOoQd+GYIExpsvjrME9BRQDsuhZWGGlbqM4jF
S+d05POWsiLcxqoPitFjYC4RDqc=
-----END CERTIFICATE-----`
	newServerKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA0XRZI3u7vNwoPrZwiJerfTVKUC1Jz4dX52rW+9ivChhL/whs
5lzBOhSeISUcKfpIwQwLXh49A7zgetA9GM6wfFO+u+rcMUP9H4uaZT4mlZu/7cjX
s6ZLIbIyKNLMuGVoOT8a3KNnPau3nB1sGUl8LPjyXO+scHGdaw1K/hIbCbceXHoN
Zz7DrgWQtIJByn80ePcjiMrRqovKQCQdZCU6KUafc/rvrrY9juWeu8MYGIvDuTGF
W11M4uB2sHvDTI6fokowE/gcnq5m7+C4jCHvFKfMWG9xtOCxLdxsFRU0UmVP8uKC
DYVuHG0GNh2xYYjifpL0GGBdbUzn2OgQkDFNcwIDAQABAoIBAF2fchCoSBx9FAgk
KF0F3oOTBGqeM7Xtu18XpIziKCuM/Ls8muDFaSF7Acuy+MnStB6GMbaaMY+wJ27+
EbE7AiwwirsYmd/zkfs9vX+vrjOFcN7qvW/xzvd63Wzd/OAXg+TCzlD9QTKRxPql
NCKBdF3t0Pe1shB42HJ3eKPkl+1Y+7pAG2nDr75dvQ7SIuRFcPSfRV8pRIJSlDV8
kyDL0f9AG8Iutt5U6Tc9aUhLHSvJMQ8U22EwRuaa8VO3g4OwxmniVK6uElSXGzTo
qkZWB5A6JzN58Ho6RosyILhAbBr/wffS94YZWr0U1TtT3ennhut9xCIvxjb1mIu2
fO0DImECgYEA3+EC58DLJ3EdgJ0Fa4447tBOXC6C3MsK6CYehM7Yh0yMFkc1ZUZJ
PVsxqFO+JWSnwyChNV6tbGsY6rq4oTf6d1aSoImBUEhR2U8rLlGMpfL3f/TeSQ/9
bgCMKci8Mca/lgWl/3Ei7hfyD5CpMIrLEnDdKGxVjbxRGrD92nAEbckCgYEA74GI
HwJ7JcoHOyuKu/9X5Yv+UOaNxDfK7spDUqjHffbo5WdWEwqoiOVcdpMvvSRtndfM
v03OKlbwkXE8oRmOtA+54V0sCTmQEAfmd8xdRQWjlFTgJGU+a7sE5QhLwi3cmUOW
P3N+/9SGRNvfNoZCnuWFsxA+iui4+EwwrVRzj1sCgYB0/XluL+I5hzO6jNNTRCve
J/56z1dVF8loTNsv3YNrGIYv8iAl/xewt2H4q2I22iWMoxV69TG88S5BIzfuD3mU
OSpAN/raQCB9ZZCUEMtlwNSzCfvKxE9T13dnMl2dyVU+iU8YcD+nmd3FYnv3QOAj
j9USFaKTgXAEea7+IgE+eQKBgQCq6SVo841Lfyq/16eN1n4zyT23H39E6Yd/9Ygr
QVPymLLDmYU721w/LGVaHFhxwcATZj6uuWgIoLfVIhhg4esKpTpBDwrwnkomlmyp
SoW4TnjXzeWRM0pi+Dda9RuSusVz/V4Hc3TKPS9/jeNwdkiuOR26lTn8SGxOi5gk
6GH6hwKBgDGRVkGIsOl37djhdl7P23WtpHOm+kzdLRaB3zOCTk30rSVeTZWEyqip
Afmnv8PUlIjsPvhS191msMZO2Z3rwh8qygWkRpa67HmRhG/BhhYQ0yDGB6v66Xkx
ZYZoj/Us3wH3CQQWzTOyw0w1CIJsYHWC9GQoq3AoJJmfRj5HdFUd
-----END RSA PRIVATE KEY-----`
)

var _ = Describe("TLS key rotation", func() {
	var (
		manifest     string
		manifestName string

		kv      consulclient.HTTPKV
		spammer *helpers.Spammer
	)

	BeforeEach(func() {
		var err error
		manifest, err = helpers.DeployConsulWithInstanceCount("tls-key-rotation", 3, config.WindowsClients, boshClient)
		Expect(err).NotTo(HaveOccurred())

		manifestName, err = ops.ManifestName(manifest)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() ([]bosh.VM, error) {
			return helpers.DeploymentVMs(boshClient, manifestName)
		}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

		testConsumerIPs, err := helpers.GetVMIPs(boshClient, manifestName, "testconsumer")
		Expect(err).NotTo(HaveOccurred())

		kv = consulclient.NewHTTPKV(fmt.Sprintf("http://%s:6769", testConsumerIPs[0]))

		spammer = helpers.NewSpammer(kv, 1*time.Second, "testconsumer")
	})

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := boshClient.DeleteDeployment(manifestName)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("successfully rolls with new tls keys and certs", func() {
		By("spamming the kv store", func() {
			spammer.Spam()
		})

		By("adding a new ca cert", func() {
			oldCACert, err := ops.FindOp(manifest, "/instance_groups/name=consul/properties/consul/ca_cert")
			Expect(err).NotTo(HaveOccurred())

			manifest, err = ops.ApplyOp(manifest, ops.Op{
				Type:  "replace",
				Path:  "/instance_groups/name=consul/properties/consul/ca_cert",
				Value: fmt.Sprintf("%s\n%s", oldCACert.(string), newCACert),
			})
			Expect(err).NotTo(HaveOccurred())
		})

		By("deploying with the new ca cert", func() {
			_, err := boshClient.Deploy([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		By("replace agent and server keys and certs", func() {
			var err error
			manifest, err = ops.ApplyOps(manifest, []ops.Op{
				{
					Type:  "replace",
					Path:  "/instance_groups/name=consul/properties/consul/agent_cert",
					Value: newAgentCert,
				},
				{
					Type:  "replace",
					Path:  "/instance_groups/name=consul/properties/consul/server_cert",
					Value: newServerCert,
				},
				{
					Type:  "replace",
					Path:  "/instance_groups/name=consul/properties/consul/agent_key",
					Value: newAgentKey,
				},
				{
					Type:  "replace",
					Path:  "/instance_groups/name=consul/properties/consul/server_key",
					Value: newServerKey,
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		By("deploying with the new agent and server keys and certs", func() {
			_, err := boshClient.Deploy([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		By("removing the old ca cert", func() {
			var err error
			manifest, err = ops.ApplyOp(manifest, ops.Op{
				Type:  "replace",
				Path:  "/instance_groups/name=consul/properties/consul/ca_cert",
				Value: newCACert,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		By("deploying with the old ca cert removed", func() {
			_, err := boshClient.Deploy([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "5m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		By("stopping the spammer", func() {
			spammer.Stop()
		})

		By("reading from the consul kv store", func() {
			err := spammer.Check()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
