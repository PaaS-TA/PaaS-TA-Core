package deploy_test

import (
	"errors"
	"fmt"
	"strings"
	"time"

	etcdclient "acceptance-tests/testing/etcd"
	"acceptance-tests/testing/helpers"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	// there are three seperate deployments required for a TLS rotation
	TCP_ERROR_COUNT_THRESHOLD = 3
	// there are three seperate deployments required for a TLS rotation and each can error up to three times
	UNEXPECTED_HTTP_STATUS_ERROR_COUNT_THRESHOLD = 9
	// the cert files are recreated twice
	NO_CERT_ERR_COUNT_THRESHOLD = 2

	newCACert = `-----BEGIN CERTIFICATE-----
MIIFAzCCAuugAwIBAgIBATANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDEwZldGNk
Q0EwHhcNMTYwNjEwMjExNTAxWhcNMjYwNjEwMjExNTA2WjARMQ8wDQYDVQQDEwZl
dGNkQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDttAUdvRf+o6js
mmnaGNteN3/O2FkkHCGyBycpcG3F42rehq5ySdK0o9az/mzxdRRuaN0+k3RiVMzD
gCkOQY5Z0W4piIWdocq9MxxeNLSqYfrAvqkXWz2nPWVxyLVukMbWZ6U58SQXGME/
KlR9RN4kMLSUtn0bexkRGoZoplpWk5PjmXDTVO/dJRf4ILRMj5Jdst+yAlHS/THf
24LkOWkTSrXtmzo3UmBAtsH4Qyc4ACx852C/bYwHl6FT8nPVatDYnTiSQqw5aDej
z4a5L9JzNLcTWAqEHofnauesmdu1+EhlMeiRNIrjrA4K2u2M/4EJEsqoWenIAeLC
HRIA0Edh8PlaJLFEIoXkJvuMOxwV6TttAdAQHqFDYpWG+rw5hS30LJqBTkOAI4DJ
Oy1dIc/3nMffqTC7FwMTayQ/o5gg6TtVJn2leifXvJ5gbAD5LI70+0rSNLFuqbx0
UUMwl6XxqZtzW9Ewfs9bbgIZ3i/l8JoBD0U8J9ePTqWbrdwVIRqusxsRr+5AG2bh
z3CFdHpz8oaLPzp1IT7RuUquKYlIrVNFhsp7bWDRg1xR+IFs6v/U/DODWiFKorom
U90WmeScvq2EpprdfODq+a4TPASwLMmP0UbiKmk7zzeyLTE2HSt+gRaTdVQhhEPF
gUa+MAvWTy970VUozbcxCjUDfjMW+wIDAQABo2YwZDAOBgNVHQ8BAf8EBAMCAQYw
EgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQUqB9YCFmIX6UyGW/nQSIJDklz
2I0wHwYDVR0jBBgwFoAUqB9YCFmIX6UyGW/nQSIJDklz2I0wDQYJKoZIhvcNAQEL
BQADggIBAOmqCjzJY6/Ui5rREdszjdFeVse6WVBX8AAI6qz0KS4zz2Ekfxt1nGMg
2iyYeSE8zFPvNUq6ydvqJRN7tiSSzbdLN6E2FtD4ss+kYtbytkcM3m7DnNlllfMx
CRJ0OoweEFeRp2xZNSZk9lOYm+jaLn377UG7PyF0P4H9nWrh3Vpz/1plI5uYll5t
ANATBZqH0G3vfo73cZDlxlY+FloFhky40Ol0DNpn+8Nv325hjY2cyNb/SBfEaYYQ
6QM1hqUFWa/bJiEDbwMzpc5NyZGdoVzbuqjD/t8aocN1bheK/QZb5EMCzAHjerBw
dPJ04ZvNUuC6oTBGMBRycWWBjq7YAMOB4joSckWSg28O1jfsUy1Kh1MjZAkuTLOq
XasUszSwxKTZtlGNnUSMVFMPosKkExhFabvMQH32KFHEUIyUM4qXrLHQe38/OLT6
LDMXuB9Z5s3f1PRIGfwwr12PsNlNoZEHjlEB7HAA6HlI5wSNN80Oby0oAPG503pG
jnJURvyB0IGxHCxHBxNWTVqUunuS34lfO2XSlAtO4Qu0lvLCXyY9FENpQxFUwEMx
0/Drjxcoy89tdqOFO0A+alXioLecU1vP/CLhmThu5/hCnREgZgZTTCTLFhClBD5B
vVSTMkEsyxrgfHuv516/abPUDTqsSMpY8U6gPV4qfObXkHvjtMwl
-----END CERTIFICATE-----`

	newClientCert = `-----BEGIN CERTIFICATE-----
MIIEITCCAgmgAwIBAgIQW2/534VFdCZ45Z0ToZ+2JDANBgkqhkiG9w0BAQsFADAR
MQ8wDQYDVQQDEwZldGNkQ0EwHhcNMTYwNjEwMjExNTA2WhcNMTgwNjEwMjExNTA2
WjAVMRMwEQYDVQQDEwpjbGllbnROYW1lMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEApPTDHjaUkjGgzA1aj21K7p+LusCMEbLN104DW/OSQ06BygiYtsMl
qVa2s02fR8rd6TQoqB4yTf0RDYcC9HT/vwNhV89RHi/4IyOEgqGtoAUQxg4Zmuap
HNNZq8RlWiZqB9JrQP19PW8qpywY+Vkyc+Pr2QFFiJ4dRq0o/Jkq3CzdKOa5idel
Y4Fgb9RJDetbyUaktbmkXLOqcI2kDNdbuOoHDFUI5T8WeSODrmGFN01nS5iynXhS
QU07zouKnBxclhhNP9wAFaP1e+Mv5FbYBCKsGdm/cqEyZNpMeqahseHyhTGaFKO2
7OZd3D0UjM84qssPuqb7in4VQrJijpSY5wIDAQABo3EwbzAOBgNVHQ8BAf8EBAMC
A7gwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMB0GA1UdDgQWBBSXOUKz
T/iYwy3TGHY+p+6hQaWCBjAfBgNVHSMEGDAWgBSoH1gIWYhfpTIZb+dBIgkOSXPY
jTANBgkqhkiG9w0BAQsFAAOCAgEAJ3mUQbrn79psWNdYu897l23AOZtNl0ywxapI
PoK139QdETShD2aBBP5HhgRqdbWIPJLzB+aRjrPrwldN+qJKXbSO74N2CcNsBPxt
1ZpEtbFr9DSYt0JxeuAiYQksRpnwcdwFg6J7Dta8Ioipb+2JXZbOeDJZz7S0I2FW
M1XV2po0OchPdTLt0mOcJyymLQUif71qBke3qG+Pl/lgHXvHjHdQ7wwL1eycCRvV
T7P3ArYrBj3IzRIo19HuBEcUMIbjfMGgS5GgVrRppavWhLEKgSHT5rr3yZz4b3st
DEIaGtetXCvgGLXHh/HJ9uO0VR/SlIFDIZn3kGmO/vq88CE8cw+Ss0kxtfOD+I/D
ra6dOP+XMaI0L8bAjvVyqpU/oj2q0bGs3lw5N/dQStvuaRtbjEm7HvhLg6JN+gIf
32M7q2aEdxkDHlIfxsyKA+yYFqeR5bOGj4bULycBmEDxwjTqiCR2t0AKI+VqZjrl
lewPb6wyCp+zUS4PYZatxSqQ4hP/sy5A3ouWbr31lQ/v+eGwVWoJj5eVQz7OyNrr
YMhUxJpNe6ndT3w2Bh6m3chlkFxFktnCmuOy9c/f4MU5NKtr0hhkk6VF1G4dMGx1
epuxfl813EBWMz4Rz3QzS6sVlQ/33f8NI6f8E5BZmRAXjYnkka+24bG8SK1JHMMs
KcCyw0E=
-----END CERTIFICATE-----`

	newClientKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEApPTDHjaUkjGgzA1aj21K7p+LusCMEbLN104DW/OSQ06BygiY
tsMlqVa2s02fR8rd6TQoqB4yTf0RDYcC9HT/vwNhV89RHi/4IyOEgqGtoAUQxg4Z
muapHNNZq8RlWiZqB9JrQP19PW8qpywY+Vkyc+Pr2QFFiJ4dRq0o/Jkq3CzdKOa5
idelY4Fgb9RJDetbyUaktbmkXLOqcI2kDNdbuOoHDFUI5T8WeSODrmGFN01nS5iy
nXhSQU07zouKnBxclhhNP9wAFaP1e+Mv5FbYBCKsGdm/cqEyZNpMeqahseHyhTGa
FKO27OZd3D0UjM84qssPuqb7in4VQrJijpSY5wIDAQABAoIBAG1XNLKlOSwCq3Q2
cc3agy3TIbrDgNUGcX0C4CUmOdBVjKCPvDKA/kjWCrqlfCwJY7j98uklQvEBCzmt
QZ8qoo9JvU+IQ1vALjmUhHRWmREV6n1twTk1JenOioTZ5Nix19yhdKianlaHhn1T
NKarok7BSIcKWb3qGLvNcfqlyIwyO/DuVlzg4h6/50pGHqSPUj+mfMVMUj323VrA
sEL2XhQjdaB2YOEsvhyZ7SHEYcNM4Pih7BmdSFjqWPtK7oXN/N5u31l7ffW9wJEB
zFqdOjnbQkyTcNXhyrK4RgLMCHYyIqK8aE41dbYk6Bx1M7G/bMsq/byGu3Koy/wW
NvNQQAECgYEA2a9aKeYm2m8cYSzL/i4vnv9LviXYnSLtgovpoALh1+q7YTu958kz
Dn1HpZuhjhG/hKDb+ENnQTP93KQR1l3GUmwn9D4tq6uVuLJBvcaXCf2eHqivXL91
kuBcYnLpVMvyTmFB3pmqsRmVCv/qoPPsAym1z6TzPzJ/ZJO9+tbMeOcCgYEAwf2B
LgyaZjKMYtLqKWvGqI1rM7cRN+mRqiA7IBL+ylARWU5IfxCp8hDHag17/enuMyg3
9ASe/rw3szDvSIanEK9Y5ccU17dJJgGqXyHb8e5rEe46uv5ZqX9S5YEDJK7DCCJo
7zk+OCwQOL2QfJ6pejy+GTwXsZw0m5fGssxS4AECgYAIXfQSPjVqGfE2Tvl8SJwt
+VQY9+1uhMQqS2RscQ/rM6uGHjy7ZwFeYjRZyjSYeFgrKd+qmDSkzfHJBElnOu1/
h4a1nZo1yf+UPM3IFJUDnkrwlL1AzF8hiRwj8JTFXJ1wo85bVv63lesjpBiJnTaK
HJVPaOCsoi1BWWho9s6fKwKBgFtIphv8ND9489Sg+S1KmO3Btjtcns6Xq0LJ7eiW
56xd5vwGOVkJh17wBFZkR/9gsAUEnOfHsOWfvfolQcP4EO9qA8QEXUtw5QvsZrZj
YXNDxMBoQNyzSY/X6TMz9T2yuvW983D2l8+o9G0uzqnFo2xw3udS/rdGEP9SeV6z
hSABAoGBAJQ5tw8He/eV9gdqWAIpgjTi9x3C9TYnZe1K/dR+M5mO1ZQhiiT1IPWS
DE2VGJL7v5SMormi3a6Fsm3/rvXOnk+OURpCLRIsJZVCUQS3bUJXStGSyI4xLEDt
r1ayBUYqXsertF0TwI4JdtIwz76+aFVWCg9b+S5Za4g2d/Czqo8W
-----END RSA PRIVATE KEY-----`

	newPeerCACert = `-----BEGIN CERTIFICATE-----
MIIFAzCCAuugAwIBAgIBATANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDEwZwZWVy
Q0EwHhcNMTYwNjEwMjExNTA2WhcNMjYwNjEwMjExNTA5WjARMQ8wDQYDVQQDEwZw
ZWVyQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQC6Ka0ZOezoPLdF
rbS7Qn3CGk+mZNuZE53RRr2N8JR/L2RYq2PlbtCpFNuqghwbh79dfn9tSyo87DXH
cnA/BZpUD1c3urvvpUVJ1mEmZZT0ZDEsJYinYY642rfCtraV0tBv/0OIqF6LvlMS
XUCQelOnbKJTwRNkVHcUvqSg1dwpXfbRzjStbmxHA8qflVEq/6o42JDoHTLGYerj
N1PRZLOUfRHXQ/vpZTR9Nd0QYZj0i+WRyTrO0AV/D3b/Uo1MERplFLEyRSNBH5Sz
qOM8QnHUETE7CqewW8UE+Wy5lhbFFZpB0jhL+81hhgw78/fsJHZVaGpwds8b+qzj
+9bzZHVj1ubtw+mcF6aG4AYt37h/6O5ILKEYdc4GwQp2LTDyYsb0i9gbLRaxA+dE
Ljkoef7d/gtNEY2Wlx++scbyb6pDxepnmc+ptEdpT2hnbSObxzyJeMCMYMfKHuTC
gSdvTIW315flMT3XBr3Gv2TsfJCHTUGrPTv/fCw/JLn9H5x7abRVFQi6eq6RMoYQ
/s2cC7i3U9d2V1QWSuc9KU3GFRryj7HS5kCjnbN/Z3MlxwdlgaZiWXumQnD8CtVu
VLXGmOXchWegtoCp2cmyWu/JcotJH4IVuu5DzztQGHe1+RTWUHiC+QLf9VzQ26/H
l3Vkp8edSOXDBwq6rMipi70PGMI22QIDAQABo2YwZDAOBgNVHQ8BAf8EBAMCAQYw
EgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQUZJ9eXTlIU+EgI20fdaKdzaxt
e40wHwYDVR0jBBgwFoAUZJ9eXTlIU+EgI20fdaKdzaxte40wDQYJKoZIhvcNAQEL
BQADggIBAKQK51lghdgHY9WjNX1Pw9NZTIZh21uHj22nZ312RNhgAq7pBdZqSc5E
G2m21mVWTnFSfCsCNFhByPEzrPesMjinGC95qOztazj0AfedpSllP+evb+vgTbLV
9xKybr6pmYeOsTMSbPMBZ8sRfgpmb3VgpHumGg1uLPGoyFdA/IhHpKD0Ww9E0fJa
lCTUdqVNQOa14NNIWajuhJgCs8RZKNl82bYz8Hyg3XTwxbOFMf/+Neb1rkrEc1Ql
+u2WMkoKL5pZIjShrp0hGitGkPK4DrrhkP4CNVCYMgJSFNijn517EBRZ/XL605Mb
4IWfqh7qWtP/aC6CavU/wrfHZUMrMim34214p4FaYbMW85p33tDyhUO0or/h7tWH
444EFjz6UpHRTKw01dPM2xm7Mdxo21M8nqwC268sUVUWTcQmbTI39nRxD3lOSgJt
2DUpMaaeJ6BVKAmM37dO8vEN4l5cA4GenmCmEc4sLyaO+AtFuT+nuO3bYPhpJ74E
81RGxDnXTmaHF3qpW1ApAQV3zQQsSD7u77iByb6nAlfqpPoiaIfK4nHTLlsQU1ef
hGPR4WvDHqWNiBtEbWGOmB3f+nDtMBeEyg/vXBtzDfC4JiYkgiNfrr3DWFU1o4/9
OTKb5SfmaHVkzeGM+RbuGk+96MV2GmvBcLdoTFKvD4FF25/PxzUS
-----END CERTIFICATE-----`

	newPeerCert = `-----BEGIN CERTIFICATE-----
MIIEcjCCAlqgAwIBAgIQQwCn9YelwW9JRU8nUNGv7TANBgkqhkiG9w0BAQsFADAR
MQ8wDQYDVQQDEwZwZWVyQ0EwHhcNMTYwNjEwMjExNTA5WhcNMTgwNjEwMjExNTA5
WjAjMSEwHwYDVQQDExhldGNkLnNlcnZpY2UuY2YuaW50ZXJuYWwwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQDYqWYL5zHeCm/75cFRQ1SBC4FBKMtV0F8Y
Z1ZR5/wGpDpKUfdVfi7wbNTJFdh3DhhlovZKMMLvUDDiyVLBeSBrAtfEW13/2wIm
sFciQyFetUPmYhPu3QNgztKbIogvC50NoGU08VQkiYE2bxcVFPlpYfH+mKUarXU+
SJCuzEmbSccprYxBDlNswFMvpIl6vDR/29yI+22KrV4D7P+NmnkamHiKTSL3zaqV
LVF1SAvdPUqImma4iyZ/fDl9smE4+83YAE8CdRnzq1UIoVH2thaVPDsf51EhDbr6
Rvk1ZGURHpIBiH1ENrjcu0u+Qm+tCrdtrzI6rh1hgrEcZgFfYyJxAgMBAAGjgbMw
gbAwDgYDVR0PAQH/BAQDAgO4MB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcD
AjAdBgNVHQ4EFgQUpM2Ml0SVd4TEqHCfDOdBCurCaS4wHwYDVR0jBBgwFoAUZJ9e
XTlIU+EgI20fdaKdzaxte40wPwYDVR0RBDgwNoIaKi5ldGNkLnNlcnZpY2UuY2Yu
aW50ZXJuYWyCGGV0Y2Quc2VydmljZS5jZi5pbnRlcm5hbDANBgkqhkiG9w0BAQsF
AAOCAgEAonmtJye5tLDUQ1hN5Y53JTrQ7SecTPviWjAal/s2bL3ZytProeJ//zpl
Wc2t1SlTZdy3PZzGM18tyRJznK2TQflib8tYi+Ink7ns1WkI+yhxyOlk+SPDWoV9
e68BYXqJlUn9LHuP53kvMd/QyeeAMxdBZ4bAOAGz4oTioZei8MoUPI3AfS9/5j8P
BzGBdtcLcprHHtr7JCeA7Aa4+fkkZQlDejtUrVBNd5jGSQKmggJKFP8/hEC7uYsH
mO5cUO4pbpJ8KuprBN+zXSVTTEY+WTWhYmYXcNzM5fwdaXGn8Wd6OfvYtGNthrDQ
DHZ5/g8igOoAf1eZk6lU6p+n3Ui5w8X9Zo2O/TbNsKAPxNBvZjkgDKVkB9RGCPIY
JV0uMv92z/7DLYK0I8/zbezSFiuyaBbDrlyFFkNx6L5ASE/kHtZkLyYSrmjSNltY
h3zqjKSlPBLJAfqfaq57sGaqxCnDoMVGoCj8YZTkydzBgh1RX5Jpb/f2TyJECf6y
PUYyrGKI16bFZMmoHSkSY1IykOjANfjD025+Jy/e//bFCNypsLCjk8Rv5RYS/C/k
sbNnMlZwRSjJW04nFPo598AAx6qgtand+Y7vsLCqYofL/x51kV2bSnKmW2WAuZbI
woSl8JiNzSJSj11iZsTKtgSPx3nEmpwhv1sF8qJO2xNG2D4QIWY=
-----END CERTIFICATE-----`

	newPeerKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA2KlmC+cx3gpv++XBUUNUgQuBQSjLVdBfGGdWUef8BqQ6SlH3
VX4u8GzUyRXYdw4YZaL2SjDC71Aw4slSwXkgawLXxFtd/9sCJrBXIkMhXrVD5mIT
7t0DYM7SmyKILwudDaBlNPFUJImBNm8XFRT5aWHx/pilGq11PkiQrsxJm0nHKa2M
QQ5TbMBTL6SJerw0f9vciPttiq1eA+z/jZp5Gph4ik0i982qlS1RdUgL3T1KiJpm
uIsmf3w5fbJhOPvN2ABPAnUZ86tVCKFR9rYWlTw7H+dRIQ26+kb5NWRlER6SAYh9
RDa43LtLvkJvrQq3ba8yOq4dYYKxHGYBX2MicQIDAQABAoIBAQCttEqzWl15toaH
v3GZNFEI0O+FDS7Qkynax+bF7ib6MCrnsQWKTotViPukaFZPRUa8HcY2PxfahEFd
Yaluoi0ifnn83H/lhHIaEKEbQBT9+HgCujle9WUi9U6WwD3M8hOtfB4ILz+Vt4SX
3sLzzQgVvEgnJbyhQdZQ5B7TdcfBrhAp9qHTVRc9WKMh9+9gqLfcYnHa5GMMEpEu
tbnXKUBrJerZLu9UetuM/vArwXwQBaFLEEbQSZcS+a7lu2REYB+nqUgEHJ/bbK4h
FuoOJgjgp61PtpSJrHcysbx7XzSFqAexqLtqqJRARSB6w2NJSz9nGtxhLpONGLJl
EVgbcCIBAoGBAPJ3qDQJHKZcS32mdIbGa3/miKAmadEFIYpX/aTK89x1dizjtbgG
dhFocn3+cq95UBcvMhZ7fhAhSpW36Aa5oN5KjNqbJEzu7mh1aUWRIN/SGi6EMMF0
QmRfn9AvpkbXcxo+QJo2/MRKqB5qlwxNMZwydEep1UaLqpa5Z6JwcYeRAoGBAOTB
CHp8IYItUyP5sPEgn7DnwZ6IqxBnmoFq9HP6urn0Snx+es5BcfJxoorL/I8CkR/M
cTSu9hCBAcfeZMXkQu4mgHDQ0yqc/JsfbQ/njGFnvhEm8Q99tFeWZ5x8+MZmUOHW
RUPrOSve7Vm7i6GFV4gdEybiKM2UagcnA2SqBTzhAoGAHRVXOq6hHh9R+sddkND6
EgRf/P2+kZDQ/hwh04N4jsgUHbxOjr1PqjTiDtTXgs7FWZKSqnmznFGx9ZVyomPf
tOoyTQJw7z11oVf8AZkv3UkBVPUMOBgu4oVJ0Rn2EudC6jHvY9AWr6DY25UjexlD
Sx4OLo5jg6u7EYs5sBVWuNECgYBdNm9PAd/hnLiBM1CvoNyRiI50HDqgj6b8z2lX
DTcjaPElM6C1BSP6Z+WU6zQ5zhD2xSboEddAuGDSYcPsg2vmgRfbYKx7c8bXKwIU
9gRU+KIReS3HYzCmdCo6MQ5qQez5aYeF+oasYsWSyAJIyf31/+r68DMyOiTOT05p
qYJC4QKBgB3eCQg3uNkw24/OSQT/AsN5qw7CZOOUdToGAMN+6eJn2TwELNde2D+I
+bTNOlvEuIqmpQmI75xfXZIZE2r+mVPARbBIYUJb1PoUSlYlhym4YJPavRboVI7R
Hhq+vaOzgaTA5rh0B/4zRyBbqc/wG+K61ZBQer1yheXwDT/ZYkVJ
-----END RSA PRIVATE KEY-----`

	newServerCert = `-----BEGIN CERTIFICATE-----
MIIEczCCAlugAwIBAgIRAOfh72gfPdheg85v6rTw0yQwDQYJKoZIhvcNAQELBQAw
ETEPMA0GA1UEAxMGZXRjZENBMB4XDTE2MDYxMDIxMTUwNloXDTE4MDYxMDIxMTUw
NlowIzEhMB8GA1UEAxMYZXRjZC5zZXJ2aWNlLmNmLmludGVybmFsMIIBIjANBgkq
hkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA4Wj0EL5kCEswUBzk7dCuQ6ONG9YHsjRQ
EcnA+Gy1sp6r4eZr0taXUe89KdYQTPZtIYrUWTNClznZPJthQivqpmO3mCgYR5Jx
Vfa5GYxSLcVWiJbE5YZlAshEBgxB6z9KlbfzahBJJZ2v+tr3egKg0DPJbACppxGf
b0IzOvLq1/nGal+nXUixi1WxWQd9HNQy8WJq/orD477fqNyZBvgyykEYZQfUTGsY
5E4xYaDhsq8DES/pYxuU//yVk8zyiieqqS+/8BhGcN/g0d0O7HK+CbpFsCkHeLRx
faJxVyHvqHq2FZoTtWd3dMQi+nSK/W685EM0i1RtFLk3TbqkYTmA1QIDAQABo4Gz
MIGwMA4GA1UdDwEB/wQEAwIDuDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUH
AwIwHQYDVR0OBBYEFMyw7MOYMuT6x4YVuB44jSJal7iDMB8GA1UdIwQYMBaAFKgf
WAhZiF+lMhlv50EiCQ5Jc9iNMD8GA1UdEQQ4MDaCGiouZXRjZC5zZXJ2aWNlLmNm
LmludGVybmFsghhldGNkLnNlcnZpY2UuY2YuaW50ZXJuYWwwDQYJKoZIhvcNAQEL
BQADggIBANHvBKQrs6RnBkkUB5SA1kRoFfdKANmPRyavqFDD9z+4T7fxa90CPw7r
LxdYmBuanB8EiYb1M8Ryud39I2WcFt108MTPOWu0MiMNalkvq6KTtzZvXTjvYhze
khuNZMFh9vD8Muuq9DU10P7xrfPOzdFRjv0326hNmU22/ldVtUWieQIjtOeSs1dQ
JjiYRoDgVLwGqdf95OGvamKrVLJig7lk2QtRYpD9COGqaqDAVKvQXjob81wDtHNw
pdL6ZBubfZRtiJLjf/WYvpSrFENqOrM2QO1X3Qe0rN9B3/VaHyzMyP7pO+LXO/V6
GVqRYL/vNBtjC1xRRTsg0Mjh/SDLHbjuiZ7yfJV7DfdK6W2wsLKC+ZYmCJTRnMPv
m5FVIcnNUbo8DwKGX1cHDGWGe+H0w5lM2NEa2rJqldZelV3cfORddbZvRhFJvqNh
A01NpSaRpfc8N3GZborTNkK2unYNz0Do5QwW5d7ALW5FkgdyVHVeuJWatANrlNQT
MguAY1EMWaHXnevq25ozRXVfc0N2oYcMtVOMJ6mQyZc66/ZusgYGJxTMpWE/7O3Q
YkyE3lJ3jXUO2j+ohOCJDxxkMtvQIstOWOWlbr0smf9llE9P2SIkWQSbw5gq+0a1
t9VglbKx1psed4STB/V/gkhCYRUukI/hHwaAtbZJplQm83Qsr3Fh
-----END CERTIFICATE-----`

	newServerKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEA4Wj0EL5kCEswUBzk7dCuQ6ONG9YHsjRQEcnA+Gy1sp6r4eZr
0taXUe89KdYQTPZtIYrUWTNClznZPJthQivqpmO3mCgYR5JxVfa5GYxSLcVWiJbE
5YZlAshEBgxB6z9KlbfzahBJJZ2v+tr3egKg0DPJbACppxGfb0IzOvLq1/nGal+n
XUixi1WxWQd9HNQy8WJq/orD477fqNyZBvgyykEYZQfUTGsY5E4xYaDhsq8DES/p
YxuU//yVk8zyiieqqS+/8BhGcN/g0d0O7HK+CbpFsCkHeLRxfaJxVyHvqHq2FZoT
tWd3dMQi+nSK/W685EM0i1RtFLk3TbqkYTmA1QIDAQABAoIBAF/RsKaJeJZ59+Cm
V76sTdfc53gkxILBcCQPoqM3+wkiICjYPY+TvyGKVHK7V5SP0JZIoLoGa1FDuw/j
cTWi2429p+bbbG7IVrtXNRoiJzDjyUQo6ywytC+5mAkGHuN0tSzt1GCK6b0+yfcW
K8tG4LuAuCfcJEIr4J14A7UUXDZQX9GYFe7/4g8UhDlXmS1VuOQd3UTP0c3WkwdN
wlXb2Vf+r94lLhxlmzoVUMS1IoXmOsCH+C5cFKq3aFVQ/vuougMkeAmmqI26QTNq
dYvGoYxVgX1T0ujlzjXTWjLenepL7G62IJkq7AtL9znSaOUKKJBwt0R82X46Q/+D
KcFJxkUCgYEA5YWGVzwtup35TGeldWaOAvHVrYRhVOUW3yogUPNpcpN180dnpJmH
hJxU0hNqdPTqHpI1a3EHQJWkB+4KYtRDYBzX4FTWocgYZgeIczGmnjwOm9vK2qh1
fJpsj/YvO3LA2x99nYDwUuy/j4arMYbnotIFv4kfq2ETd5W9R5HMj58CgYEA+2oA
EWKZE14uBe231V2JINQWYW7B4CchgtNcMbH3pJVw+SpoSVchALmVYv/Uh+UVHevM
ebcxbBta/0YNW69QpK0pk7QBayG8zXMZ6JODH2+n894wLA2mevgz1Lt/3DOmlpRH
Tsya5Uoqbi7hbXau98pQbRXxeViHvotnDbAMiwsCgYAiNnIdBMpoO+4SVozSYDQg
+j14vPfpOLDdGSFyD6aTPqnhVq57WatauBborZ47ytovLmoqFtIW7Xdi+zevHabh
Z8tCFENeID6KzuqnCSqmAZvH3c5yI5RHu5kdKHxH50YaI6qM1NB9++5eDZvtKQfU
PGxA7ca7vB+zvq1VQsV0TwKBgBrbewBgcQvRnscBWwcPA+we2/kylMF2TK0mGQ4x
/ct2L2hesF9NUHg8WwoFXFXcEgJtQx2phT0QOwtUF2847jt5SBzAOPqR0xJ7fkQL
JhHAosd5b9n051jxlM/f68vBNMWXN3rifpWJ87hrh6di61QLJ8ZPdslIvM+NIsgi
i2R7AoGAU/7HPWVoCTJU/cV974eb5Pms9/oo3QpSDLhkZxdMmVvwHMWRUHholUpr
mld8opIvWTtcqkB8F5X47OZINxfu7SAhlKwFMtIuw1we8ZzpmU3Y/Ik/MjeFQJhU
SqV4zLqA2Vk+crbtUsAyP8qUoaeuoKUIJR7tkiPBg1QrBiUqSEs=
-----END RSA PRIVATE KEY-----`
)

var _ = Describe("TLS rotation", func() {
	var (
		manifest   etcd.Manifest
		etcdClient etcdclient.Client
		spammer    *helpers.Spammer
	)

	BeforeEach(func() {
		var err error
		manifest, err = helpers.DeployEtcdWithInstanceCount("tls_rotation_test", 3, client, config, true)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() ([]bosh.VM, error) {
			return helpers.DeploymentVMs(client, manifest.Name)
		}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))

		testConsumerIndex, err := helpers.FindJobIndexByName(manifest, "testconsumer_z1")
		Expect(err).NotTo(HaveOccurred())
		etcdClient = etcdclient.NewClient(fmt.Sprintf("http://%s:6769", manifest.Jobs[testConsumerIndex].Networks[0].StaticIPs[0]))
		spammer = helpers.NewSpammer(etcdClient, 1*time.Second, "tls-rotation")
	})

	AfterEach(func() {
		if !CurrentGinkgoTestDescription().Failed {
			err := client.DeleteDeployment(manifest.Name)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("successfully rolls with new tls keys and cert", func() {
		By("spamming the cluster", func() {
			spammer.Spam()
		})

		By("adding new ca certs", func() {
			manifest.Properties.EtcdTestConsumer.Etcd.CACert = fmt.Sprintf("%s\n%s", manifest.Properties.EtcdTestConsumer.Etcd.CACert, newCACert)
			manifest.Properties.Etcd.CACert = fmt.Sprintf("%s\n%s", manifest.Properties.Etcd.CACert, newCACert)
			manifest.Properties.Etcd.PeerCACert = fmt.Sprintf("%s\n%s", manifest.Properties.Etcd.PeerCACert, newPeerCACert)
		})

		By("deploying with the new ca certs", func() {
			deployManifest(manifest)
		})

		By("replacing certs and keys", func() {
			manifest.Properties.EtcdTestConsumer.Etcd.ClientCert = newClientCert
			manifest.Properties.EtcdTestConsumer.Etcd.ClientKey = newClientKey
			manifest.Properties.Etcd.ClientCert = newClientCert
			manifest.Properties.Etcd.ClientKey = newClientKey
			manifest.Properties.Etcd.PeerCert = newPeerCert
			manifest.Properties.Etcd.PeerKey = newPeerKey
			manifest.Properties.Etcd.ServerCert = newServerCert
			manifest.Properties.Etcd.ServerKey = newServerKey
		})

		By("deploying with the new certs and keys", func() {
			deployManifest(manifest)
		})

		By("removing old ca certs", func() {
			manifest.Properties.EtcdTestConsumer.Etcd.CACert = newCACert
			manifest.Properties.Etcd.CACert = newCACert
			manifest.Properties.Etcd.PeerCACert = newPeerCACert
		})

		By("deploying with the old ca certs removed", func() {
			deployManifest(manifest)
		})

		By("stopping the spammer", func() {
			spammer.Stop()
		})

		By("reading from the cluster", func() {
			spammerErrs := spammer.Check()

			var errorSet helpers.ErrorSet

			switch spammerErrs.(type) {
			case helpers.ErrorSet:
				errorSet = spammerErrs.(helpers.ErrorSet)
			case nil:
				return
			default:
				Fail(spammerErrs.Error())
			}

			tcpErrCount := 0
			unexpectedErrCount := 0
			noCertErrCount := 0
			testConsumerConnectionResetErrorCount := 0
			otherErrors := helpers.ErrorSet{}

			for err, occurrences := range errorSet {
				switch {
				// This happens when the consul_agent gets rolled when a request is sent to the testconsumer
				case strings.Contains(err, "dial tcp: lookup etcd.service.cf.internal on"):
					tcpErrCount += occurrences
				// This happens when the etcd leader is killed and a request is issued while an election is happening
				case strings.Contains(err, "Unexpected HTTP status code"):
					unexpectedErrCount += occurrences
				// This happens when a request is made right when the certificate files are getting rolled
				case strings.Contains(err, "no such file or directory"):
					noCertErrCount += occurrences
				// This happens when a connection is severed by the etcd server
				case strings.Contains(err, "EOF"):
					testConsumerConnectionResetErrorCount += occurrences
				default:
					otherErrors.Add(errors.New(err))
				}
			}

			Expect(otherErrors).To(HaveLen(0))
			Expect(testConsumerConnectionResetErrorCount).To(BeNumerically("<=", 1))
			Expect(tcpErrCount).To(BeNumerically("<=", TCP_ERROR_COUNT_THRESHOLD))
			Expect(unexpectedErrCount).To(BeNumerically("<=", UNEXPECTED_HTTP_STATUS_ERROR_COUNT_THRESHOLD))
			Expect(noCertErrCount).To(BeNumerically("<=", NO_CERT_ERR_COUNT_THRESHOLD))
		})
	})
})

func deployManifest(manifest etcd.Manifest) {
	yaml, err := manifest.ToYAML()
	Expect(err).NotTo(HaveOccurred())

	yaml, err = client.ResolveManifestVersions(yaml)
	Expect(err).NotTo(HaveOccurred())

	_, err = client.Deploy(yaml)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() ([]bosh.VM, error) {
		return helpers.DeploymentVMs(client, manifest.Name)
	}, "1m", "10s").Should(ConsistOf(helpers.GetVMsFromManifest(manifest)))
}
