package helpers

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	etcdTLSPersistentDiskValue = 10024
	etcdCACert                 = `-----BEGIN CERTIFICATE-----
MIIFAzCCAuugAwIBAgIBATANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDEwZldGNk
Q0EwHhcNMTYwNzExMjAzNzQyWhcNMjYwNzExMjAzNzQzWjARMQ8wDQYDVQQDEwZl
dGNkQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDFRpOpM2mdmYeB
KXPk2E5k50VeAVTwAcFmwfwsWVz5Z764etvpVaOL63gBeJnUbU5luIW0CMFZZjdW
YrRqC+ZnI6+JmGLsU7V0dbKTA1WrZoErs9z75MAege6QwT+aJz5Ge/FsOnREHGW1
vP1KRJRX3WmGkznYT+wKuR+j4aJ0r7n32rD1lzvYTjh1QFGO9Jq3g2wiIWuJ0Vd0
UfgL0ASOGioykf5I23tBtnKSCiRSAe2UNYM1X0lHb4tWR7bGAEqDX1EK98gOa09z
SNYjMooqh9/tGN7Mm6QXlMNEFgy8+LgLZuBQuAFp0H8oy0sH8ftrFXbt7tly7Aq6
3t2VIX7wxZLie2lCJTYa3QrG2n2RIh4nUH2dAqmVF3kgXpyW1fkta+kp3gMwTWCc
K3mz+/igMAlKRYgED6F+94GfCPmV00Wny7kMMACOGbJzEtwTWKMZfFnvPvo05n8i
dWHO5OAmkqvxOuMGPow6kG5Lu1wlXx/f0S4JZSxGYvTu7YCFTu18qifjI3CQ/qs6
JZRtX0U26zMbCLntzcDCPeEttlSpjoPk5TvAAixffCK8YIFBcBOe5RTUYzWvB4ZT
SNXwb9Pfeu9E1HSrljyGlGb+hd7yJffEbkx8W7K5ZSotEqQuDeGML+mPAYfPbXD3
6Mio215D2fYCGS7+NBhZNdXf+NHM7wIDAQABo2YwZDAOBgNVHQ8BAf8EBAMCAQYw
EgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQUZg9U61G8/Hu6672Rrp0xF1R3
3PowHwYDVR0jBBgwFoAUZg9U61G8/Hu6672Rrp0xF1R33PowDQYJKoZIhvcNAQEL
BQADggIBABeaknYHK3FHzBGSDo/3b7W8vcn2zp4bnaUf5lUwGFwHG6BGlkGRsPw9
GweIK2ZUss6FM98QUv7dxo8WjT6dvvCJV2dgUCz5YEcfOAiCTqV97YkUh0MUNOzd
180vnNhp5xc18537yomm/+bAK+SiWGlYhpqlkZN3X9wOu2d0xmdRzBGMxdhPVj2M
hiUDPTcUbyeFgaruCWKo2TxLg25bS/+SSWgnhx0Xdg+/rA2TJl04DT4lDhx1xcHo
TQzzHx+fBYsszmpb74HmMlviH3QhnD1mvmSLpQ8KR2yXF5ujpekAVfh3oR232Xum
B2Xm8K+RdPbUBjYuBchVyTqpZGnmorWl4b2Ppyay/a1UOJpwODPhZ40IKXbZb6ar
y4V+fgNOH67fOAc2HdkNaxrrEG7fW5+OUJL55SxKbAssHZvbaRx7JCbcbKe230Xi
/Tfuy34iKSytPddq9EHShtj9j7ZWLpXbWsBH/FcPzWwbbSkkNFeyBK+tnpRCDV3U
fC6M8rAbQfwOZrlt+kBJPTh3QeFbv7MNKIyEQljCh8T+5o1PEZBsdRdTN/rfTExI
RZGVPdK4BGzj9niSbxipugAbQRoDOhYXxSkGh5tVHyFCXg/u4RwuORipYZrpqOir
63lHxTO7H3me6fqNlGVc1H4UMqhC0p84v11/4eXrX9rPWUKJzvrg
-----END CERTIFICATE-----
`

	etcdServerCert = `-----BEGIN CERTIFICATE-----
MIIEfDCCAmSgAwIBAgIRALuL/EQbWwerlEUeoyvyOqYwDQYJKoZIhvcNAQELBQAw
ETEPMA0GA1UEAxMGZXRjZENBMB4XDTE2MDcxMTIwMzc0M1oXDTE4MDcxMTIwMzc0
M1owJjEkMCIGA1UEAxMbY2YtZXRjZC5zZXJ2aWNlLmNmLmludGVybmFsMIIBIjAN
BgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAy1oE2PyeFr/eunnW12/LcIQOuwEk
p4bq+DPpB8EhbwqX6Jnj/CgfLXLt9sGXHVI1CXsCnCjgn0T5hO8lU4ShIn+cIbAo
JhDi1BVph+C1rpDNwrrtXz2w6rtgDhKVHzFuXU9id8vsMmcxxx15HozZdxQ3hQVc
Ybz5HYsNt8SimyBwWIVirvkpR9Zl8eUeJs3x7WriwSMiGuBlfUe5jeml8Yp34Yqr
ltoxJtVR43gtvrjU28kIRUuOJ/gbFqGZHVmaSVKnrv3ZKjkriOa+B76tItG7tI8i
pG8SZ/03Uk3oevUOd6cPLTG0/IYpxvTQ0rEmHfy0JU3whUW0W9hKv+v87wIDAQAB
o4G5MIG2MA4GA1UdDwEB/wQEAwIDuDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYB
BQUHAwIwHQYDVR0OBBYEFD5XmnQPpbWj+73JJyXSKKfPR483MB8GA1UdIwQYMBaA
FGYPVOtRvPx7uuu9ka6dMRdUd9z6MEUGA1UdEQQ+MDyCHSouY2YtZXRjZC5zZXJ2
aWNlLmNmLmludGVybmFsghtjZi1ldGNkLnNlcnZpY2UuY2YuaW50ZXJuYWwwDQYJ
KoZIhvcNAQELBQADggIBAIlqMICytmOjle1xMImqZHqrkLlmvmC39SaDIXS5mE0I
8gGngt4YkOlcAEy5bqEXN3x8DVvGqBxk9ydpY0Bi05ysiwFUACi3sxmhAsSIgZl6
BA3puKGdqAl/ue0BAc5z622Rh7ckuM4XWNkVy5jK1cLSbDFDTiQDzRMUgc+6LFYm
5vn2rT5h/ohfjdoiGVy7wr74jIsOhtfwBAPFxBaM8fBGBUC3LrIvahm1AvIg/jUc
mmfkyVutTihKlyEF0olb9KiQLTe1RctTh5QWM4jEc8DRERcSPQcSouMZ6KPz2bzQ
Z/xXjpOaiZTg5BnxHh2tVUnkcE3jbo6lSZNsl3bOjfx/GbaXMP28Mh4aUORQoe10
NWraGmV/KxL+1tJ90+exAU9T2RXxzIPVPwXe60ycQXtov+/WHq7R1IatH4W2ljgb
UHQnHNqXGqMVbfcP3YDdxEWQnHzY+rFhgjwn3lCSGjzgxetWdQspDBlUOyMjBhFm
mQOqrFueP28bB0BzqItlflSOdPs1561x/iXP/q8Yi325gYjWaDavYu4KpIVcmrLm
W2vLYyFUqk1q7LuBbb1ccbS3RlQN0NPp8Qahw3icGSnMgA18pyqjdUd34xkpWeDU
Q84KuYUyNSoMr84G0cad87dQqiMSpntYC6xd1mfh9ocedJ4285l7GAZc5WknDa3w
-----END CERTIFICATE-----
`

	etcdServerKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAy1oE2PyeFr/eunnW12/LcIQOuwEkp4bq+DPpB8EhbwqX6Jnj
/CgfLXLt9sGXHVI1CXsCnCjgn0T5hO8lU4ShIn+cIbAoJhDi1BVph+C1rpDNwrrt
Xz2w6rtgDhKVHzFuXU9id8vsMmcxxx15HozZdxQ3hQVcYbz5HYsNt8SimyBwWIVi
rvkpR9Zl8eUeJs3x7WriwSMiGuBlfUe5jeml8Yp34YqrltoxJtVR43gtvrjU28kI
RUuOJ/gbFqGZHVmaSVKnrv3ZKjkriOa+B76tItG7tI8ipG8SZ/03Uk3oevUOd6cP
LTG0/IYpxvTQ0rEmHfy0JU3whUW0W9hKv+v87wIDAQABAoIBAQC+P4/9egplekjk
6YyIrj0FHWeyqVUjruQyJk7ERHoFK0IICcH0bY0NtlLP4zp/4iNgpUdB1jSgjaVs
K1kelB0063KlSeumAXJVvVqoFyGjGjKHFt9xlYPpeDhbsiL1tgdtIRIcxhpK5aT6
hqaEYH4sHCv9NZDCmEvwyeGhpkQDIF+bB6N2ilS4Hc18MaIYp1PfgUubLVo859ON
i3bk13VuDDiu5UjD7A1rZt1EAAXXLIK4uv0RHho5t7RBrjQTfYIxruKGuElIzzFj
iqCXrH/+ynjXSmp55FlDIcTjRcmBfIAsnGulvst6uYTuu3GBhiHC89OFqMUZpMr1
2iiOtWlBAoGBAPlygg1rQ9yCd5hivF0nBSEGVntHPNHBB3vmOK3Bkbf/qzhDeK2C
PXS75ulDYfl4o07NxwSd6xaAhF0ZF1w/IzhEYFZKUHU5QcEZvpvdPf2npIeOBk8H
ZXQY24L8nb3/+OIEF1HV4mtoJnILvNF1cCsCuHsM1Zlq5I6D49rWZu+fAoGBANCx
hnMs0glynXdqeJD9vbDmdPbaAEcjq1VwWaOO5Bc9aWmBzdqrZsLqqKElXxIqXhCE
T12LylZWJwBlU29KE7ixLbjj9+KDUIoUUgbzXCuY070KY2AzUTxG4qzSxE/11kOT
2agjd92Tps58Oss+ud+amrcQfF7PDOoLEljHHbCxAoGBAMxBl3yleMv3iTaeot8k
NG72YZpQmtym0xoBSif9ePTRxcIsfYSWQPx1YH9hTbiZsB+3IGAHb5jdY4VYJmjC
ynQoiTofYAKc/9q+2fWHFFvACll1UnUj+U83i4eWkxQhpgpsjyvTl7ObdN+t/M8G
+vI9KBKaT81wWfbYyJtJNMDBAoGAGgNW1/ppP+Y6fI0X9DK8t1UylSZ9TGDE1YSI
l9uS0NbF0fHtH+mniHpJhLSs0g3X5cUoQ2fOU86vU9xNdxzLsoTbRyWbW2+01VFN
HDKvdXu6QOEPnAkpghLv5EztTW4+Q/Qk+FFbepISA8D2byklcBrMWC9E4Wh7mpzA
r7I6IDECgYEAoWozAc0D6gxUZDB0UkaqqOmZXiYSqX6pQpDYKKmhZAEZ3Zc9st1h
+OUOw7247M5UtjU6BtkOeqzGsJ3zn+ZhYwO9SOQqVYU5Pgdab8HzBqjAyYGGXMx2
acn3MqQrNLd8JctUTfmfCNfI9b6VZpy4hX2GcSYp2cJ+g7VbO908KZw=
-----END RSA PRIVATE KEY-----
`

	etcdClientCert = `-----BEGIN CERTIFICATE-----
MIIEITCCAgmgAwIBAgIQV8tfNpAYWIWgxTpUnMs+dDANBgkqhkiG9w0BAQsFADAR
MQ8wDQYDVQQDEwZldGNkQ0EwHhcNMTYwNzExMjAzNzQzWhcNMTgwNzExMjAzNzQz
WjAVMRMwEQYDVQQDEwpjbGllbnROYW1lMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEAwavqTmU1NYlD2Cb+gPPKZrX1x77cTXRtQbb9evOjX8A5UMeylAIx
SqEewKtT5LpwbexC52pMOyFSP8JivNriAwUitr+0eBtATTE82DYHy2kMssEXZj6O
HZLos4xg33aNZyfdGcPsTjpn6uWbcIrmIF16S/TEC7dSsgDm+jT+1QeXIN8qKhPd
qqKPDdAY6rJLdkc/8HX/BRmWLvcKWh2at6ViWXUM1fkmNiq1quw6ndulErrIDGGE
+2nE9C4/xheRJAurH3E4m4k3XDMGUtHdOUiPRcqNyLa0rezAP8sqPuvZMp9IBx/j
KvrSfeo4ziNQOQ+rwDGkwGXPp7p5ctmJ0wIDAQABo3EwbzAOBgNVHQ8BAf8EBAMC
A7gwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMB0GA1UdDgQWBBQvOopr
6+l8IHGPknOq8BAnh5h1XjAfBgNVHSMEGDAWgBRmD1TrUbz8e7rrvZGunTEXVHfc
+jANBgkqhkiG9w0BAQsFAAOCAgEALB70zcBosBGm8YtndE6vVfvNEyXT0HtMq20J
v8v0WLuP4ndje7EKYL6Ui0EsedPqWQ8SLHMOihP1lDoQVPJyk5dNIkCNHtk5FpLb
Iio4UNLLEZLooBzYN9Nz6iaM+53S2raLkH7echpePpviJ5vQiRNo8ZPFHhJhyVBr
nPqDm+pQxQ0AfqoVuBxukXfw5GZQXgkD0ZOnZmzpXQy98RW3EFqIEGWL7D8pvZiI
qCd81ao8E+xz1Jpc4NYQssQVQr9lpI9mOa74OZ9WNapkO+oonzvreQhYYLpVlr4a
uthiwwH2/hC3tKz+98fhn6HWJ2AOpZnhUT4ZybWU+PfdDvLLKhq84hOyVv4aD7Su
rbE1qq9lHscgRToJYajAn2A1+m0O3NXAGphzyVucHq32sQRB3nP9Dfa9B1QLkltV
DuaqUkH7PZG0C9LP4ul+gedvUKQ+depXDbZRnI1ZjD4YkeMWcUFDQe/nFRPSE78Z
6pdC+6CbksDaAJ9dfCXM5u/qcd8lx9XLKwPR22MC25B00HKMG6d0oIY5G8OXC21e
zf8EYr67/BsGMZSNagCpAuiS74kbKWz6EgslW67/Lkt5aTk3dA4pX81tI92YGTDk
OL86lVtrqTyi2O9lCce8e7CSwXZQUeHfovnIF4/ap0J+VNuoIFvWIRbv+UGJMfWs
0Ke5roE=
-----END CERTIFICATE-----
`
	etcdClientKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAwavqTmU1NYlD2Cb+gPPKZrX1x77cTXRtQbb9evOjX8A5UMey
lAIxSqEewKtT5LpwbexC52pMOyFSP8JivNriAwUitr+0eBtATTE82DYHy2kMssEX
Zj6OHZLos4xg33aNZyfdGcPsTjpn6uWbcIrmIF16S/TEC7dSsgDm+jT+1QeXIN8q
KhPdqqKPDdAY6rJLdkc/8HX/BRmWLvcKWh2at6ViWXUM1fkmNiq1quw6ndulErrI
DGGE+2nE9C4/xheRJAurH3E4m4k3XDMGUtHdOUiPRcqNyLa0rezAP8sqPuvZMp9I
Bx/jKvrSfeo4ziNQOQ+rwDGkwGXPp7p5ctmJ0wIDAQABAoIBAQCJVahUKS6fJRRI
DfbBaJ8pUHTWTQCZqCrlw6Zh7qz2dC/wmXuHuzpK6pANHjDibFbXjAcxZM5jy9Aw
SF6N/0Tv0U95ed22odRqKLU1uLK0SmznwUcfiNJsJEYBNFVpgP7qnHkMEUPbgU05
Y9Ji7wwa/U6A2DPA/yGT+lHQMY5XbsuID3lj4SkbdgVxHDcsJ/5AgLFAo6fnQ4Dq
Ooi0Mxt6QB2LP1FhIo3miRnHXsBqgYw59E8wQkn1F2oaQlAzxj9DjWrr+SlafB3m
r+chRyeF+6iu6C5eHSxYn9cXFrXWzWhIVRzxtjbKa9pnMyVgOyawEfjkELqcSsmy
Uxt4GefpAoGBANfEXUWEea2SmSo5SQTeQAHAzVuxKRaAnNaHoLNtpolvL60tbC3d
zEQofzvhgaYY7wDjdOhT9ffQlz4WuoWCrsU9VooIm4wW/mPvESPQCo9MQnkaL6cO
RIbgB5d0CavBU3NAiSrfMyixBuMiYIwWYw9uysOdZmc7/E+JXhVVL02tAoGBAOXI
0o1/0OHwuKZTkBJ/qElGyBkiHdLQbokZJFatYcdVfs2xtkWehgzVHmkQz18paR69
/0Z1KzcxXrl39x2Wg8b7r5n+qDW3XJ+TPeQzdBfMQJQX3l0AVI/yN7k0H1VEYcOC
nkG9RyB7DRDqX0LQN5jIaiaFM4C8/nah2J23yCV/AoGAPCTESwhuUm+2ugpVzEGX
XeW8WH69kUQwc6xCo0FBVrXjeEZdDTvyIF2ZebuWRBJXLMw6XjhpK7a9MdVsEKMo
zFoYsUlM8nPGXVzaTj1DdEYxkUg3WD2l5GK2OwVhXLr64/ltQsIMpJ8T6GRAvUvQ
OREM/BH35XbXEeSckR25ndECgYEAvrGuqudL/nW7h60Jf5CZpFYtcU4y2eVIFlbb
JWO2Jar6FNJKpfQs4zFqj173+c1wA6dB1sMeHivGpLy+Q7vJmLT+whnolsuY/oU/
c8aPrcBAR6aXTy8a/mrRe82ZwzWAvLQFiiD+iiIUcdlPPS93ND/+eAFLAKfXtbQT
BLCkVRkCgYB1y+YKMEdnCDXb3lfrMT5QIA43bvhbs4/1ouwhSMaG47jYolMhT4T4
EVJj1k3/uNItbFRdFauug1onTydMPdrUg3gjcb3hJqU+UbyLKin0Cb4EGxNpj+3J
NaxryCFORb8ZfDW0OVeuKk/7boJwWoF2gxYv8AhfB3YkeGb97rySbQ==
-----END RSA PRIVATE KEY-----
`
	etcdPeerCACert = `-----BEGIN CERTIFICATE-----
MIIFAzCCAuugAwIBAgIBATANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDEwZwZWVy
Q0EwHhcNMTYwNzExMjAzNzQzWhcNMjYwNzExMjAzNzQ4WjARMQ8wDQYDVQQDEwZw
ZWVyQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDZM39o1oPzu2Od
ikJYtvcFhZVkohdm6kbxL2vS5JqUWIorKJg9o/LO/oTmy2SqHdbdVavTs/aQusZk
MmYyNIsSsYOYF7II5wyC03K9gsT/oZnivQAT8oEt2m4T4LBtg4Spv2GcWBGjrZfF
8G+ZB4DxVehFNru4h9zAKNFrWsoVsbb2AHh6hP1BOX6SSITTKBbowpnRD1+/IeFQ
aaR+IN/ufRe0Cqw8oYeedECk/Ho6xQ7lspmXiFNgI5tuDF+HU8vfmP7mUiYfq6Zx
IERHJWqOx30P3naPcEMiaKYV5nr928gPSMCoCPArIH9zMVkR8Q3dzM2Ub4BtVKGq
RZe+T2LzKAXGthAKuxO1wqw74qeK3RxWHqnhUd1q2IVjbUGd28KkOh38GEkg9Gyy
re2JHlPB70J8Ylv/RVyPeljP/aOsuB51IDoiyOQxU4IrFNBhSi5GIc+ZplG7LME1
XZYQM+CqicaOokXOEiXxwhIBOIUY3qCMuyuhY/LxbznCzk8md6syotB+o/iAZHKq
EonPPaOrqS8Z2DZmL/MGGEk+K0Uk93zw7kqYofPO5K1LATSVsX0c17WDbq2CXC6y
vJR50y6DR2Da+Pb+7NXsyBlMP7EZF2i4MZ7huB3TxtoqWeGIBwE51PO8A6ABIlqe
C4jG446K17mTpFalyausELsWiDLDhQIDAQABo2YwZDAOBgNVHQ8BAf8EBAMCAQYw
EgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQUYFXzdS+enntQoy6++x658hC+
lxYwHwYDVR0jBBgwFoAUYFXzdS+enntQoy6++x658hC+lxYwDQYJKoZIhvcNAQEL
BQADggIBADmZWTCUp6bLopTSOkfOwO3B8yDzzmVMNd1JfpeKpymWdZXFGqepcfij
HAOSfZKcKEKPJhtApds0eZKuiUtT3vM+lxpf/kMzLwh/0SEtl6qJOGDtbpfw4v4A
aiGYDKVq/xRYOy/JHYkKLzG40C6cUifIUBr+yCm6Kqd9BlezjAL36mCXyMId9r/j
axDR48/B1tESqdjN0Klcu7TGbD6OD1JIzF1qMfiKrJ445NnYHZQcvu02IGzEntGH
xiHcvuPXDMZlICeyVOFd7OdW2rNVs6XuVkxUhjAtR5AFAFW2UkP+NF9ti/zQ5Pva
FA3FWKbPYMp92OgO9dHi7oqM1NqXp8b5olU1CxkKVoGQbIvRcJhTTIXvGpE3t7PK
HK2hDJ4u91CY6Oh0xWzbZnUTKRBXIG7xf1E56B8nW15Q0OJINSxKpsanZ+j792oJ
CcKIppmJaESLqr2f2ntP/PfB3mViDwcM+80HXXM3/YYbbgMtfZ2C80/t5Gn09kTJ
dNgeo4EFze6MeWb34v1S9gQBWEpl30LEKu0xearfAOp8xO+/C02oQG8FQ4fNWNSB
612482nBsoXeOgHsj62WETM8mKzSIemRmF+s/r5AAVK2QfJES/cFWAEiVVeyhX2f
gvaFmf+g+aqtd6cxiwR0FPQzhosht8L2YHzmY92kiknNrTX7P4dr
-----END CERTIFICATE-----
`
	etcdPeerCert = `-----BEGIN CERTIFICATE-----
MIIEfDCCAmSgAwIBAgIRALsvsb6SluruOIS+vfum47kwDQYJKoZIhvcNAQELBQAw
ETEPMA0GA1UEAxMGcGVlckNBMB4XDTE2MDcxMTIwMzc0OFoXDTE4MDcxMTIwMzc0
OFowJjEkMCIGA1UEAxMbY2YtZXRjZC5zZXJ2aWNlLmNmLmludGVybmFsMIIBIjAN
BgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAubihRqzyPQyItRXsK8JuGbFHqPOF
hSFvOFSdirtp9bUJXtQi6XSxGPQiepI/c5HQ01nWB+5MtvPLzwraN3v9/IYycg+8
CLRIAoipiydp0ezeTSqI0iQ7iykC2TZRjh2F+DG46mdq4VLUyRYMSdUOgzOTS28g
WUD49w/hj7sUCd2bRN3m8hooWEeDxFEMP+MXDO3GRH515y5EcjL/mh3dXCSffdjs
0NpNvzGljOST5pwhUeKAvn28TkXtN9OxyoLSUIokRBnQstfq1sriNP5quXuMv8VX
DmuUkLBoLy2S9mc5MzwSlyiZPkfnJJeyZy0t+ourSlm9LQ6MmsUKUB6VjQIDAQAB
o4G5MIG2MA4GA1UdDwEB/wQEAwIDuDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYB
BQUHAwIwHQYDVR0OBBYEFOyNa8mTVZWPhxaJLExtsrc5vQ3EMB8GA1UdIwQYMBaA
FGBV83Uvnp57UKMuvvseufIQvpcWMEUGA1UdEQQ+MDyCHSouY2YtZXRjZC5zZXJ2
aWNlLmNmLmludGVybmFsghtjZi1ldGNkLnNlcnZpY2UuY2YuaW50ZXJuYWwwDQYJ
KoZIhvcNAQELBQADggIBAKRhd9HB3bUnR3KJHYZfieD3EClVnyadu1sRXp8RodjT
3RAUwEghOzvuvDlwUyM+YbV1iAuoP10Q4efrbRrCOov7hZe1VX9JQ3oxoO0Su5OT
efw2WS2tVbIQ0Gs2zs7M2jf2pjVzpbYnYZOmSF/lDgfAZnSUdtoyKUqY65y0ECv8
VMCdwgUq0bJ2ZOeWjTbAYfVJpSz/ebYLuqEyOMKEgtahpEeh9Fu1VhQ1Jb9mNs82
ocM8P8kSMjEa8ycB6wcfPxbalc5cO1HKas2feoS7Nh54xdghTNdo15kDgZtYhBit
DeOjSE/Alv1Ct8veK5UE92f3Q5QkM2Uu5Gh6BYmzdSQLZR5QCA+m7eBSMdPVHKEI
eeQn/0NdKs4ccAvUr3fTftv2w/1g74rsFAX2EfMAnY6Q26g8dGVRxZX2mR32rToD
TqEbcdNXwaZ4lK+AWjf5YIV2jVHEUjeEBxc71ci9z2IqKTDQ6zKf/+C09oaW4NlJ
livynC/67nnAXGjFF19B2V7N8T+wjBKnkCCjZ/AXqO77xlKMy7Ba2ScSoImBe1Ww
B4ohJpZmiIKyB0ZllJ5DDuiDFcx6TbVh65X3CkTA01mlFSa8EpW4fw6KcHooQ1E5
UO9YxkmYDaYHr5nENOxBCdx8imxgsK2M/AQObGPEAbolmk+S7EVNKT2f6oTH8luU
-----END CERTIFICATE-----
`
	etcdPeerKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAubihRqzyPQyItRXsK8JuGbFHqPOFhSFvOFSdirtp9bUJXtQi
6XSxGPQiepI/c5HQ01nWB+5MtvPLzwraN3v9/IYycg+8CLRIAoipiydp0ezeTSqI
0iQ7iykC2TZRjh2F+DG46mdq4VLUyRYMSdUOgzOTS28gWUD49w/hj7sUCd2bRN3m
8hooWEeDxFEMP+MXDO3GRH515y5EcjL/mh3dXCSffdjs0NpNvzGljOST5pwhUeKA
vn28TkXtN9OxyoLSUIokRBnQstfq1sriNP5quXuMv8VXDmuUkLBoLy2S9mc5MzwS
lyiZPkfnJJeyZy0t+ourSlm9LQ6MmsUKUB6VjQIDAQABAoIBAC2YsK/DYi2u2Blt
anBL6fAQ4EEQmKVY0g+IZq1a2ebjLYvElVWvupMfzR8+rKlZEWXyVmPYE6mPLWiF
h5G7tV28LMJoLogMiulkWAu7/3T0rJdRbAn7r0q5JOPjPB6iDPQkcPvLrCfpyCge
a6Hs1wVLMkyA2fZPx1AQ7BX3njHVdUBK6pI7X/kraMJxu5wBHKbZzLj8BguHFGjd
lV12ux0p2+tE2KSgh24i5QcNmO/45DtVOV/byKpuuMbKltRj6slYaSj0knY4fNnW
xEXMnHih4suUsMTrszlyzNUz7irl2lnwTTevYpUZVn/0+TCO6eLLrM+vgt9GJy+K
ACzRVRkCgYEA5nHrc4ZzFE5ehb1TNq2NQh51OIBOADHwEiHB6h/rZZnrokq6h1QP
YRtHEH6kpDuWVi8PE/KxSejF57BKbFDPJ+8OliLSgYEDX53YG1WeIJFJRygcP3bA
TQ5xE4eLj8NADHPNEQMxjirupLn4VwGLFFUxHFI8Q9lU97Uk/Lwyh2sCgYEAzlEN
EkCx9TIL1mFzaXrXA72menBD8x8oPbT/CKMFn3DvsCSgxLxypujuUZBrZ2KyOnE9
0ru+DjBOG2XxfcBM/gSkRy3iSXRKCrE0vXHNZkx/myLCITVl+Irm/bhDWuKlQvZQ
gNSVec4TWuXny5LRQHjf/X5lB88cSHfCnX3ALOcCgYAjEx3KNKGZaqA6bOmYfevt
L2OaGPVGVFN8/wRb1UXn7fiOeB9R77pzhkpXuV7n3GXycjEyURMo87QDorKBL/+H
zXwD4AL4USGpUQYOiwaJYHOtz+4UvsdgMx2E4nGcjRRXkNyahUjqoaA3FFM3MvXv
P1Q9QksH7LFhDoI1sZNjRwKBgExrgy72nbQXxIC0+f3hDVGKZubFPLYKHWq15x14
3PVQ7MdO2enlb4ZZkyTNHKtfyGqTVXYAKoaw582INioBF8OjToI7Aa15kI9jUgi1
5YH15fI9rrCESfAE60ihfvlkKBikie8eTvueFFc//1rNWArMexM3RQ7ebTh+e6zA
TnWTAoGBAMPOot6UEHS6peBOknihWINYo3km3Mg7CB09jOGn1NRx+DMFBU+nTIF7
tK6EwSle1hvxSWw8KnDT/WG3TK8o4Cz8kbZxe4YGb4B7xufkAmUyFXHjy8Z+JK2U
lJ9b6dTbzS8niDOqIjVUo7oiIrUtUWSiZmTuEqmFclMcA4+QfXyQ
-----END RSA PRIVATE KEY-----
`
)

//
// postgres does not have a metron or consul_agent for this specific test run due to HA related errors
// consul servers do not have metron agents due to a random routing 404 error
//
// postgres and consul servers should not have metron agents on them
//
var jobsWithConsulAgent = []string{
	"loggregator_z1", "loggregator_z2",
	"doppler_z1", "doppler_z2",
	"loggregator_trafficcontroller_z1", "loggregator_trafficcontroller_z2",
	"nats_z1", "nats_z2", "stats_z1",
}

func CreateCFTLSMigrationManifest(manifestContent []byte) ([]byte, error) {
	var manifest Manifest
	err := yaml.Unmarshal(manifestContent, &manifest)
	if err != nil {
		return nil, err
	}

	var etcdz1Index int

	for jobIdx, job := range manifest.Jobs {
		if job.Name == "etcd_z1" {
			etcdz1Index = jobIdx
			manifest.Jobs[jobIdx] = convertEtcdToProxy(job)
		} else if strings.HasPrefix(job.Name, "etcd_z") {
			manifest.Jobs[jobIdx].Instances = 0
			manifest.Jobs[jobIdx].Networks[0].StaticIPs = &[]string{}
		}
		for _, jobName := range jobsWithConsulAgent {
			if job.Name == jobName {
				manifest.Jobs[jobIdx].Templates = prependConsulAgentToTemplate(job.Templates)
			}
		}
	}

	manifest.Jobs = insertTLSEtcdJobsInfo(manifest.Jobs, etcdz1Index)

	for jobName, manifestProperties := range manifest.Properties {
		globalProperties, ok := manifestProperties.(map[interface{}]interface{})
		if !ok {
			continue
		}
		if jobName == "doppler" {
			jobEtcdProps := getEtcdCertAndKey()
			jobEtcdProps["ca_cert"] = etcdCACert
			jobEtcdProps["require_ssl"] = true
			globalProperties["etcd"] = jobEtcdProps
		} else if jobName == "etcd" {
			globalProperties["advertise_urls_dns_suffix"] = "cf-etcd.service.cf.internal"
			globalProperties["machines"] = []string{"cf-etcd.service.cf.internal"}
			globalProperties["peer_require_ssl"] = true
			globalProperties["require_ssl"] = true
			globalProperties["server_key"] = etcdServerKey
			globalProperties["peer_ca_cert"] = etcdPeerCACert
			globalProperties["server_cert"] = etcdServerCert
			globalProperties["ca_cert"] = etcdCACert
			globalProperties["client_cert"] = etcdClientCert
			globalProperties["client_key"] = etcdClientKey
			globalProperties["peer_cert"] = etcdPeerCert
			globalProperties["peer_key"] = etcdPeerKey
			globalProperties["cluster"] = []map[string]interface{}{
				{"instances": 2, "name": "etcd_tls_z1"},
				{"instances": 1, "name": "etcd_tls_z2"},
			}
		} else if jobName == "hm9000" || jobName == "loggregator" ||
			jobName == "metron_agent" || jobName == "traffic_controller" {
			globalProperties["etcd"] = etcdConsumerProperties(true)
		} else if jobName == "etcd_metrics_server" {
			globalProperties["etcd"] = etcdConsumerProperties(false)
			etcdProps, ok := globalProperties["etcd"].(map[interface{}]interface{})
			if ok {
				etcdProps["dns_suffix"] = "cf-etcd.service.cf.internal"
				globalProperties["etcd"] = etcdProps
			}
		}
		manifest.Properties[jobName] = globalProperties
	}

	etcdProxyEtcdProperties := getEtcdCertAndKey()
	etcdProxyEtcdProperties["dns_suffix"] = "cf-etcd.service.cf.internal"
	etcdProxyEtcdProperties["ca_cert"] = etcdCACert
	etcdProxyEtcdProperties["port"] = 4001

	manifest.Properties["syslog_drain_binder"] = map[interface{}]interface{}{
		"etcd": etcdConsumerProperties(true),
	}

	manifest.Properties["etcd_proxy"] = map[interface{}]interface{}{
		"etcd": etcdProxyEtcdProperties,
		"port": 4001,
	}

	result, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func etcdConsumerProperties(addMachines bool) map[interface{}]interface{} {
	jobEtcdProps := getEtcdCertAndKey()
	if addMachines {
		jobEtcdProps["machines"] = []string{"cf-etcd.service.cf.internal"}
	}
	jobEtcdProps["ca_cert"] = etcdCACert
	jobEtcdProps["require_ssl"] = true

	return jobEtcdProps
}

func prependConsulAgentToTemplate(templates []Template) []Template {
	needConsul := true

	for _, template := range templates {
		if template.Name == "consul_agent" {
			needConsul = false
			break
		}
	}

	if needConsul {
		templates = append([]Template{
			{
				Name:    "consul_agent",
				Release: "cf",
			}}, templates...)
	}

	return templates
}

func getEtcdCertAndKey() map[interface{}]interface{} {
	etcdCertMap := map[interface{}]interface{}{}

	etcdCertMap["client_cert"] = etcdClientCert
	etcdCertMap["client_key"] = etcdClientKey
	return etcdCertMap
}

func insertTLSEtcdJobsInfo(jobs []Job, insertAtIndex int) []Job {
	var modifiedJobs []Job

	modifiedJobs = append(modifiedJobs, jobs[0:insertAtIndex]...)
	modifiedJobs = append(modifiedJobs,
		etcdTLSJob(2, 1),
		etcdTLSJob(1, 2),
	)
	modifiedJobs = append(modifiedJobs, jobs[insertAtIndex:]...)

	return modifiedJobs
}

func convertEtcdToProxy(etcdJob Job) Job {
	etcdJob.Instances = 1
	for templateIdx, template := range etcdJob.Templates {
		if template.Name == "etcd" {
			etcdJob.Templates[templateIdx].Name = "etcd_proxy"
			break
		}
	}
	etcdJob.Templates = prependConsulAgentToTemplate(etcdJob.Templates)
	etcdJob.Networks[0].StaticIPs = &[]string{(*etcdJob.Networks[0].StaticIPs)[0]}
	return etcdJob
}

func etcdTLSJob(instances int, azIndex int) Job {
	persistentDisk := etcdTLSPersistentDiskValue
	return Job{
		Instances:      instances,
		PersistentDisk: &persistentDisk,
		Name:           fmt.Sprintf("etcd_tls_z%d", azIndex),
		ResourcePool:   fmt.Sprintf("medium_z%d", azIndex),
		Templates: []Template{
			{
				Name:    "consul_agent",
				Release: "cf",
			},
			{
				Name:    "etcd",
				Release: "etcd",
			},
			{
				Name:    "etcd_metrics_server",
				Release: "etcd",
			},
			{
				Name:    "metron_agent",
				Release: "cf",
			},
		},
		Networks: []Network{
			{Name: fmt.Sprintf("cf%d", azIndex)},
		},
		DefaultNetworks: []DefaultNetwork{
			{Name: fmt.Sprintf("cf%d", azIndex)},
		},
		Properties: &JobProperties{
			Consul: &PropertiesConsul{
				Agent: &PropertiesConsulAgent{
					Services: &PropertiesConsulAgentServices{
						Etcd: &PropertiesConsulServicesEtcd{
							Name: "cf-etcd",
						},
					},
				},
			},
			MetronAgent: PropertiesMetronAgent{
				Zone: fmt.Sprintf("z%d", azIndex),
			},
		},
		Update: &Update{
			MaxInFlight: 1,
			Serial:      true,
		},
	}
}
