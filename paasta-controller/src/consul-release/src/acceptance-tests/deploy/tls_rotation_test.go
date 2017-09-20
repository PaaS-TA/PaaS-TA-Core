package deploy_test

import (
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/consulclient"
	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/consul"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	newCACert = `-----BEGIN CERTIFICATE-----
MIIFBzCCAu+gAwIBAgIBATANBgkqhkiG9w0BAQsFADATMREwDwYDVQQDEwhjb25z
dWxDQTAeFw0xNjAzMjIyMjEwMjdaFw0yNjAzMjIyMjEwMjhaMBMxETAPBgNVBAMT
CGNvbnN1bENBMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAvQ4z+dtT
rBeUSxc8JK7heuBzHTOmn6v1AoXDkbT5oJdEh6vfqnXL0FwRdaFuXRG+7XCjlMXT
t2ouLpezCqrHKfE8FbPLqU3RSY/M0VVu7bAqD1tifhQ1TcrNJM8Xlm/V9Qqa/5ve
5aZZBCgQ2LeAQ90ZBEBT9bHtwKUbCjFBtJupdP+itHmMjpJOkMYQKA9mvnf6WoIl
UJhTxQIILLdvpeyXVsNKC4p2PlE11N6YSBiFYZ+xtV5ezhFkR+N3ZmIWu+VX3vVw
z/hNPTzsaBdRh+Wqoq7rIIy9X+/pNxNgLNSXZIhoBP6luCVd/NQtjI8xzoo+ithT
BVPwXn/UefHB7f7AgWoi4MrxV79w0dSTT+EIYzBWVw3SodP6tnKIWMcrC+xDPu0o
aBL+wW/DK03phFnF825M8Jg3rGNzD2U1nmDUTNcp3loY5oT4qTbrtxfxdVSf2bSN
6D8HRMxfQ4dnthDlbNR+Zr02OOGMwbFN7OMkSsWhYpti+K2TniFEdh5jBkgBY17k
hjol8caq6EjMVT3n/cDMsGOdvh9oM6cScSHgPjzKCGxp0qMX4GRO0xxIAjIC/JTG
W0DrvYljoCLBFVl/+R+bQ707IbzVUznSogXk7+icxN/0LFJnfXh141yiK00m3zyc
Tl8tSYBEQnXsFuJa0iZu+RUIXsd67ZjJbzUCAwEAAaNmMGQwDgYDVR0PAQH/BAQD
AgEGMBIGA1UdEwEB/wQIMAYBAf8CAQAwHQYDVR0OBBYEFMCMTG07AEv5dpOw4je0
kqyUCS2RMB8GA1UdIwQYMBaAFMCMTG07AEv5dpOw4je0kqyUCS2RMA0GCSqGSIb3
DQEBCwUAA4ICAQCOvFDey8SM6ChK9NdH/UHRPt91cxIG31bxOowvsYO15bgcwqUP
MX6aIWixUZb3SwbQkpv0TBPjs/EDPjm3qrSjp9yVnksrID1otq9uSe0U+2SE+PvY
cJJ2OuIjZy4oym23ZP37cSb/SFs6YuQVkrrnqWVswdgXvtGYZ/GUxaEIt7J7N7/w
U/m+G/ndKJAlOVqUisdJSBIFg2mMQN3tmZLClYJ8+VrNStYsNWcWuPc6i4RKs+jP
ULG+L4/i5si4r0uL81lEiCOfwxJobIP5X6mTSEKompOmRKvRpuoB1XWB7tKYb5fZ
h3pLRX3WLWhiYh2o/ICeMbZhnoBnUZF81Wrju9n+FltFp9rjwIsa9Kz1eA2Pby6s
wGpFETofcgNfoHN4b/hLi7EUb/YLG9ATvjoDOBt1P9mYbPM10pTQ7LAd0LdVksFK
kk/LFzv/wvGebv2xTH/tGXeOsXyhRAheT2z3n4PwJXMc1uVsuPMAKshY2H98KfLF
NdwXeP1Fg5walHBKDu+s/mn+o7CeXICUBApmx6w5DvOrBKJ7iRvTtq5nC3AyWN6f
wzQ9d+zkyWxUjTpPzTBc7UU7Ick5lJUgQB3QkmQNyZc2enUI5q+cS91X/dDgjsei
+bYKVaTvkg66MOa5+tVUAz9qL2Q9SndVRzOK4wPgign/I7v1NxEXXOlUEw==
-----END CERTIFICATE-----`
	newAgentCert = `-----BEGIN CERTIFICATE-----
MIIEJTCCAg2gAwIBAgIQdTV+O1prfdOP9ImicpoXNjANBgkqhkiG9w0BAQsFADAT
MREwDwYDVQQDEwhjb25zdWxDQTAeFw0xNjAzMjIyMjEwMjhaFw0xODAzMjIyMjEw
MjhaMBcxFTATBgNVBAMTDGNvbnN1bCBhZ2VudDCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAKfYixt1UjZ0agvOjhWjHa5JWCTQxxsqClHTXLwyNrFE5eOQ
easgHL0x/EEw2dgoWtChbAuyHPUAM90GZLRjooMZbPNdl9GhQw7Wz6UCMZCQXl4e
3GwCm1PpgBHbnwTIr+pJZzMfC2wvkH6NILyJBnWif19T6MGKXFEsjzyIeK8PL3/f
hEfrJM4+ff8oRLfbGrm6NrGwzFbvhVbHD3bMvYZaDdzitOOz1yykl4ABexD7Pm1m
ISr1woiTpdnvdstziq4sYnXwvnNaZw2JZAE0vn/5VSeeWyFNQYJf48NokTIwJu4N
b4udRCP1KzCFwWC9kBX5svLMUGm3ExCEnuXaLsMCAwEAAaNxMG8wDgYDVR0PAQH/
BAQDAgO4MB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAdBgNVHQ4EFgQU
LI71VaszpVW4LTez9zTv4AybmA4wHwYDVR0jBBgwFoAUwIxMbTsAS/l2k7DiN7SS
rJQJLZEwDQYJKoZIhvcNAQELBQADggIBAFyhNT17KemtSRmtks32uRo2fTT8SN+V
PAd/ArqOXMarr4nTRhz9nKVi7ZEV96PRxlX97O/WUfWeK2qIS1fqJ8rKGNYMFv/b
LaakhFhbiGEj2Z8BI2svelx7zqZFqzpzlKBK7f/a7FSqXMpKR8ULQYBITHN3EgTM
orZlJM6FOAOsdouzSk4Ft0TFMNXIT0mTDRssKo9ceWXH6xhwC1T84nCnv87ETGCA
NNUrG2UZweDFed8jmHFjPP78l0I6UjW2VavQIvHqQT/xkRIfjlXiNJNsaa9Pm9wo
42LWmy2JZbQjzqslqGk61Hxikoadvo8CHkk32/2ZaQOTKalQlbkUyIaPyd8nLfYC
tw6Iy6EE7EtfNVS9A2N3i4n8gIyiQDR0rOinHTX7G05DgT97tbxOCxFCnFrRkq4q
CXlpH6G5YhtV4s+l97uSB0MFHtVbqimKHL/+1w7S3yP0LkXh66YpBrHvNzItTes5
f8LFjyouweR8WHUd+qRxS3SpqEmS/yh2jAF1Y0HPz5T+Cb0XfpBA8s1mTfAjs5++
wrzQoMsRgnRSNPizd9JVMLYhTdYcot9wi6fl9z2LucpUXyW2lxuQ+sKVNSTwyTC+
jGFGjHOWdhLZFZuKY0Qxb3Zi1N2FQYwRXOnKcJWpzng8E4iXL6BxFVCuD3tkx1Sr
4lsVCjrV4+DN
-----END CERTIFICATE-----`
	newAgentKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAp9iLG3VSNnRqC86OFaMdrklYJNDHGyoKUdNcvDI2sUTl45B5
qyAcvTH8QTDZ2Cha0KFsC7Ic9QAz3QZktGOigxls812X0aFDDtbPpQIxkJBeXh7c
bAKbU+mAEdufBMiv6klnMx8LbC+Qfo0gvIkGdaJ/X1PowYpcUSyPPIh4rw8vf9+E
R+skzj59/yhEt9saubo2sbDMVu+FVscPdsy9hloN3OK047PXLKSXgAF7EPs+bWYh
KvXCiJOl2e92y3OKrixidfC+c1pnDYlkATS+f/lVJ55bIU1Bgl/jw2iRMjAm7g1v
i51EI/UrMIXBYL2QFfmy8sxQabcTEISe5douwwIDAQABAoIBAFT1eixS7WNc99S0
IB15rGts+q3f8/ifBgw3FYi5Tg/a1RakKcHiBkoKBCqnZI1Sl+1k2ADvjlLBYH8v
Xkgk6ry7YPeq108n9n6LYx2eB6KqQOoZau9NPnxyA/6GEW7leo33y8IHo8uGI/i6
zOhB38ApmZmSKo3U0DfSe0pjtdq93dg/oeXsKRox/WiFFGmEUXyzhc4R5SQtGZRE
10NUzc6uMQKnbJkOBvAY8BiEKXNmTFljHpJ3mCwRtCQOkekY70JwGe4cqwurKpyR
lJ4stLqPN78vGkjfY+GaAi1S0mOe6ZVjH7GSt43Q1BMXdFGS0ZRR0+kdkYaoIjxI
kPWCQLkCgYEAw99GtEIG57XUrxOqRnvnA/xgoJwsDmYjT3gx/DfekqxkU/tx3U72
fkvchIjt7cBR+m4RR/Ejlypum9opwO0ArONip/4A2V4kPlLz5DgErqHySEpuxMqS
5lo5NwHftA4BIiyROdpf9h8NKfOFRVtd3BHRi5Qplwr4+tKw7YV1liUCgYEA217O
B1m44D96BmbseM7tl0qQwIYNR4FkOWC+ZXCFIOxnXuuN+OhVmSkjYa0fCQwK4aMt
fdXQRByG5+wSnpbF9ijxGSnjpYWl+l3OqohsVVufww/lcAQIp+dV+Yns0XRG/ny4
XyyYLjpt49Ywuo121Ij9DUlHVzrQdF2B2dgzGMcCgYBxh57RqFucPjZSbBGL3REf
rE7NiPe4ONdKnp5KVI+7cBSO4PU0kyooNgxQ/ZT68zgQ8W8uxcQdQEjwKNl+q2By
1TE/segIFZroTOh0ZUvBdLib0hi2E7xlq/HxwjJJiLx7dF2QrNRmMcVNhYq/kp+q
iOFuB6i7lW6O40QNyAdJyQKBgGo0pwjV/nTLJpfM4rXGcS7rEdOz0uAIm+5PkT5p
UHrVGWLSJiUYzsBdM10JxNnLc8U0DEU87Bzdts6383fGRUddIQTuy+EKKIZjPjg/
3jshJeL5YjpuKYaosG4kwXvSkMCKv3SMkYzoCuXggC0BakORovn4vUpVFjEQSFqg
mnRnAoGARGnRj1zUoFMqOq0ZoGdYOo0SKf6tzykgb5gtn0XW/UDgndPIDC67dDkF
6zUb/BGUS8oJK55SFzhCLKloV6IdNF1eh19G6t1NJDL1kkWC6q5PbrwpIR25Y9X/
Hjein3kxxTeuhiQtmjRhaWkk9O+7jEaXcM4VxUSmYUKx43Ph3+g=
-----END RSA PRIVATE KEY-----`
	newServerCert = `-----BEGIN CERTIFICATE-----
MIIELzCCAhegAwIBAgIQMX+sGacHctEjGOAst7x4KTANBgkqhkiG9w0BAQsFADAT
MREwDwYDVQQDEwhjb25zdWxDQTAeFw0xNjAzMjIyMjEwMjhaFw0xODAzMjIyMjEw
MjhaMCExHzAdBgNVBAMTFnNlcnZlci5kYzEuY2YuaW50ZXJuYWwwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQCkiQDMqSxTs7oVxR/rLFfzMczFiSWeEH02
3L4+jA2vrQGuv/SOLID3rd4e2UdvwWVWzuQHUVqOUP4e0s4caDxIisobYFWwZEJc
Eex/aiC0RcyMYMlqT7Zi8RqID/eK/OJgFIYSjK57aedl8A3J3MqSrcoZBiiCiHCo
JEPdnzgKVLIjVJgV3xpjOS90qMlyxepfbCg0vZasDwl6v6vCvY3yjviMCn3qBQAe
vmlDppvzhVXhD7mVr8BBVlNXHYxr6xpYWwwaiXKDyD0Rf42ItLJQ/r68FE+aco9f
5d8BZxGOKz7CblA6wz7lU4TZd9dygGhlEU6Hz7a8kwDU6wHODsFnAgMBAAGjcTBv
MA4GA1UdDwEB/wQEAwIDuDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIw
HQYDVR0OBBYEFEGOH76baqgGrfIU2YjHyUfKgLgbMB8GA1UdIwQYMBaAFMCMTG07
AEv5dpOw4je0kqyUCS2RMA0GCSqGSIb3DQEBCwUAA4ICAQA4LiztZji8ZGdiK4gk
a3w9pjP2BHJX3gtQ4lfpWAFRMTURfy/hnKyfMCzw22cf/lHW6bfdmaTMwxJvXiTa
5GeWj8nMLzd16662OXNTfSWQpomnr6+P9qrzq0l1xAQ++Gn2m4C8GXBfquJ51Pz6
7l+Ams4fqdDLavW/B6ETodIgGYL8AUbvBvx+HbCTgaBY+83417yAk4JkXvZ0P1zp
QLj0iP/sK6bkrjIkn6sgGBUTx5AjSMHGurR412/pm76cppMLQ1o55j2PwCiAmvNn
O3xkkov3B1m9AECq7Tl3FrDlIML4rfB51n3JUcMY0TQDuLpiE+eWSK325DhO2PdE
lkJzzr/hheX+rJbHrPdBVNie3mfR93NieCqwTCOGur6N1SkqKEnNdyXBUMVCluBZ
ONqJmKZA6YKbCAL+89dRSiu40QXVPAL1Z4JP5kLSVD4XBNriMao7ni4VH75ZldK8
6ES7vsLsnYZ2mIsVhyKibZJj8MqEZ5e+nbUp+/ewcEwTc8v4DXsl+/l4gDWO5BM5
7N07V+IRF387TciNf8qg+pgQCNqX74GoUnC0Mmip0qZjss5BxlEhCsqMbpjpg573
NSFgZ2s7WPoVLZeLIvNFdMi84uJ+twWY9lGbw2AsrGHyXoMlu5YSLNpRpkAQ+o5f
m85mY1B5/YNeyPhiQdxdEFNBdg==
-----END CERTIFICATE-----`
	newServerKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEApIkAzKksU7O6FcUf6yxX8zHMxYklnhB9Nty+PowNr60Brr/0
jiyA963eHtlHb8FlVs7kB1FajlD+HtLOHGg8SIrKG2BVsGRCXBHsf2ogtEXMjGDJ
ak+2YvEaiA/3ivziYBSGEoyue2nnZfANydzKkq3KGQYogohwqCRD3Z84ClSyI1SY
Fd8aYzkvdKjJcsXqX2woNL2WrA8Jer+rwr2N8o74jAp96gUAHr5pQ6ab84VV4Q+5
la/AQVZTVx2Ma+saWFsMGolyg8g9EX+NiLSyUP6+vBRPmnKPX+XfAWcRjis+wm5Q
OsM+5VOE2XfXcoBoZRFOh8+2vJMA1OsBzg7BZwIDAQABAoIBADZ97Vba3IRoLMQT
Aiw4BnTT1HbDokLEQUQPPa6nYc0B5mHCzzLbCGd/HOZonaEkkvR6FslZpz0lE9SP
ipWb7AM2fBMvB5Ig0l19zi6wrl4mE8WWNH7SIZyJL3lKmHheonahtXmlQBA9ldaL
93UYe7qydhFtmbMJjw4Q3K0kk0HQHQbzXZbd27gdG15auefqYpvxsZOY7u5Y/t7e
3GNhEY4LYzmUwodSIcHukDFhH6km7Yw+Z36pDSF5Vqn268XOM1elaeyK/RRmqbWg
PDbRyYyiIe8UV7kTOmjO+YjLdGAS/19fDUZVVK/+hBzYFn3/SZN4mSK4hGErRrsB
LZWDDwkCgYEA16r5Og+9UtzmD/pUc9SoOLbqQtf8J5pPCXZEzzB4+MZTN00ffJ2t
xeVAUxbjkeBxgYcWxIjhHKhc67zUa43YnCY68y/w8mXe3afV6LYXlIW7p91meAqY
H/nxCyhCGwhjWVfftI0yqNbbE88OH9s27W++EqKj1XwfSC0o/UpLkBUCgYEAw04R
pSpuIVQYCd1m+n9XNaueGi/A5ZnWBJJfAv68l7sZ4LnIGpKp5PALVhcLX+lpizYt
O834qlZ7pmGeLodb+Jbdf666Tti3WdGYu326hAkjzbe9+rFftR1/g+VaEln1tmwq
++eoTf//Bw0Hauz0BbM2/aDQga867pCSKQvE7osCgYA+EUGKuS7mYxZ+8K9PaptD
PzkqJZi3GQy4D2Z8LloSVplqZ/Kw3Xw+YNzjTMoPmIVyHpup0i7fHYEogv6rOXZm
cgYzKM/yIulB52SDhaxBnT9Fb01nLL1dLoR1jo9/0iktdEG4Z4510ufXypYpCuDC
8o7ENDRsYz1pez25r6ERhQKBgFReakL2ZGLjaAsC6NR3pB3cSE05qdPFs+1/qamq
j5/gRJqOxwGrr9blV5BWHiTNuTlZKws1vCEhgQLsEqA4+yMVURQyT+t1tScI4zjD
ZIpbRGs+38PnUdf0qTw6HMHmuL2YVq1BcrRXTT0nhLfNKtE3jR7dlJUhNI0QSQOQ
QP9nAoGAQMBWb/rlN4MVHesmKb1vls1Zq8fZd7Mq9nOK37Za7x9t+/7Xx3bfEAm4
nr2+BCcD01LGzoF7p0k9XRyS30RcFa+kABfCmt3ojSEcIrmxlLnxp63CPBm6hoN2
kz9FT4jnME5mUOg+0p5XEfoZUhklll3HWTU/hWVwJzplIRAz3mk=
-----END RSA PRIVATE KEY-----`
)

var _ = Describe("TLS key rotation", func() {
	var (
		manifest consul.ManifestV2
		kv       consulclient.HTTPKV
		spammer  *helpers.Spammer
	)

	BeforeEach(func() {
		var err error
		manifest, kv, err = helpers.DeployConsulWithInstanceCount("tls-key-rotation", 3, boshClient, config)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() ([]bosh.VM, error) {
			return helpers.DeploymentVMs(boshClient, manifest.Name)
		}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

		spammer = helpers.NewSpammer(kv, 1*time.Second, "test-consumer-0")
	})

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := boshClient.DeleteDeployment(manifest.Name)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("successfully rolls with new tls keys and certs", func() {
		By("spamming the kv store", func() {
			spammer.Spam()
		})

		By("adding a new ca cert", func() {
			manifest.Properties.Consul.CACert = fmt.Sprintf("%s\n%s", manifest.Properties.Consul.CACert, newCACert)
		})

		By("deploying with the new ca cert", func() {
			yaml, err := manifest.ToYAML()
			Expect(err).NotTo(HaveOccurred())

			yaml, err = boshClient.ResolveManifestVersions(yaml)
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy(yaml)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		By("replace agent and server keys and certs", func() {
			manifest.Properties.Consul.AgentCert = newAgentCert
			manifest.Properties.Consul.ServerCert = newServerCert
			manifest.Properties.Consul.AgentKey = newAgentKey
			manifest.Properties.Consul.ServerKey = newServerKey
		})

		By("deploying with the new agent and server keys and certs", func() {
			yaml, err := manifest.ToYAML()
			Expect(err).NotTo(HaveOccurred())

			yaml, err = boshClient.ResolveManifestVersions(yaml)
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy(yaml)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
		})

		By("removing the old ca cert", func() {
			manifest.Properties.Consul.CACert = newCACert
		})

		By("deploying with the old ca cert removed", func() {
			yaml, err := manifest.ToYAML()
			Expect(err).NotTo(HaveOccurred())

			yaml, err = boshClient.ResolveManifestVersions(yaml)
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy(yaml)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifest.Name)
			}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
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
