package component_tests_test

import (
	"io/ioutil"
	"log"
)

func MetronCertPath() string {
	return createTempFile(metronCrt)
}

func MetronKeyPath() string {
	return createTempFile(metronKey)
}

func StatsdCertPath() string {
	return createTempFile(statsdCrt)
}

func StatsdKeyPath() string {
	return createTempFile(statsdKey)
}

func CAFilePath() string {
	return createTempFile(caCert)
}

func createTempFile(contents string) string {
	tmpfile, err := ioutil.TempFile("", "")

	if err != nil {
		log.Fatal(err)
	}

	if _, err := tmpfile.Write([]byte(contents)); err != nil {
		log.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}

	return tmpfile.Name()
}

var (
	metronCrt = `
-----BEGIN CERTIFICATE-----
MIIEJDCCAgygAwIBAgIQGmIUmXoWc6jvYr6+aU2b8zANBgkqhkiG9w0BAQsFADAY
MRYwFAYDVQQDEw1sb2dncmVnYXRvckNBMB4XDTE3MDIwNjIxMDgxOVoXDTE5MDIw
NjIxMDgxOVowETEPMA0GA1UEAxMGbWV0cm9uMIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEA6n5KMubwzD1ZKjTS1ra5vCDaaa7WKJFdR9+jCLo/86QoNOBa
WvBgv5dXj+Z2BZ6TYbzIIcZrtmdqOPsScQLp85x+4gFz7n1gJh9l65rIS7M6ejBW
Q46jk9+vUIz01pG49EB2e++KXgwGSxl6u5arIqtN6UG6jBebasStOJ8rO+CHKlWF
m43WJXi3kZLSlHjTPLMaCO0JFattEnj8YsseuvrvQh+qSPJg/V+FHEgzrPCZm8kS
xQf9gp0OwZF8letCzIanMvF1lqjWJY3l7tIPlNWcY/VY3JZUzfPsNhIbtY426QX8
Bo1F3jdBsQh8c9DCP4JDkSLK8Tk4zaLnxoSWJQIDAQABo3EwbzAOBgNVHQ8BAf8E
BAMCA7gwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMB0GA1UdDgQWBBTt
gDIsLc6gpr+YY0NGGkH2KgCfGzAfBgNVHSMEGDAWgBRuhaiEo9f4YphIL+RIYNo7
gGqgszANBgkqhkiG9w0BAQsFAAOCAgEAYeGPEPD5OSS/bngEjJxxmoR6yBHQc1x7
bRfu02JOJa9sdMg3Yc6VOXwz66uQGB8gTG+7+o2nrLaqNBbFNmx587jTrbO0DZQt
fXfscUCuw0E2iGtHXw+xOOHw47xSmkA3lk3CCJNQ/+CAF4BBBHp8wGsidNZiwEkK
PmNZ5SH+uGHnIC1jYjLsAnTjbSoKIGb/KaudYM3tWsYFDPlZ2x94Kox6fiXSsnRn
OxpYEiKcS1UmD84olekLKHncRUaH2gXy5Cj5AVecaQD2tEYwP9vTDVXmpdsSwiHU
XW7/sCIbcovl6R/h/OKbHsR1DPirDwt153iF4jiObnIRUeD9tKKolxWZCw2Q6Ein
eOt7hlw0kM6k5JsXWa2TJ5Np3Vamq3GDP8Q0OzwS1U1nRgRyQgITWveEvi7bJC+l
CnuctrHVrpo9y2TxIujr/lMI4sHVS+6GSLOaKNCf8Rp/eLuVXy20u75kblXVZOQ6
uR2O1/5GBOEQZzWJtvxEvBlUfLduC/UGrJl4dLzn+6lxSnWk8NwmPBjFzDFIVjbB
h4tQj+2qR2XBCvHsdOoSOoxvS+Or3zpNF9lHpYgfLaeM52cS9wK35DyrpJGgjhAS
ilh0Yhxp1ktSAkBdHHLjkWX8yDngWXp48USiBwOcmzNZeRwqekqkDD1BIC5pSuNB
gYgBTkWvXSk=
-----END CERTIFICATE-----
`
	metronKey = `
-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEA6n5KMubwzD1ZKjTS1ra5vCDaaa7WKJFdR9+jCLo/86QoNOBa
WvBgv5dXj+Z2BZ6TYbzIIcZrtmdqOPsScQLp85x+4gFz7n1gJh9l65rIS7M6ejBW
Q46jk9+vUIz01pG49EB2e++KXgwGSxl6u5arIqtN6UG6jBebasStOJ8rO+CHKlWF
m43WJXi3kZLSlHjTPLMaCO0JFattEnj8YsseuvrvQh+qSPJg/V+FHEgzrPCZm8kS
xQf9gp0OwZF8letCzIanMvF1lqjWJY3l7tIPlNWcY/VY3JZUzfPsNhIbtY426QX8
Bo1F3jdBsQh8c9DCP4JDkSLK8Tk4zaLnxoSWJQIDAQABAoIBAD12oMg61D8UjXyM
n/77oi93hQhSdXvorkSaj8dH2l9oVcmWTNitTQ6rAp6LT8AlUog5zVNdCPqknKkW
1jydAOmDyZY/vz1xy3PyoupghcOh1OAWL2ZBywqFhRRd/gcH5yzOgL/3h5MjH7sr
kIn+8hiCQkeznMv/nBMePjErN0/YB9/aAZdo06tJEUaoIairR/Eiy6n2OwNC8t9C
U6fNWhWYFvOCOReMHxUhpq3sDntDA4zlckVE6uvMVSsQYYg5/zDAbYFb8sNV6AKf
k3Q3Wff1EHRgQF4xCWVzu34N/LqSpL2Ie+NXZdh40iOZ5DhvSmByu7L9q//CU4QZ
ZgU41bkCgYEA+TF+sgWbRU/DycZ+QTLjbUI/jwZSuAo5q02W953OnUCfXmmzF3gs
OFoVbEJDTN3q4D0LCEY+JPwg/KbeWO9kW6f3Ft9MFmSgjvPf5WMPKt895xwIGcJ2
dSNqLQhGQ0xRnRBp7uCzibu0dLHpWONOfgDBrHMQZQnCwyzO/i/K2T8CgYEA8OYA
+tkuwcYbguKEL15Zv5FQ7USQaBeLxnIJDN/qYrv8qCEpeONayipT2KgkullT5Fne
gv0Okxj114P60BEPH351xzOz2wS3yHPxkfhrnrTVAxTy3mBa9FgMm+RsN6lMWTmb
ww3HPN3yvcoY2kSKsIC5rFfYLExoyh7kRiA0s5sCgYEAoiZ/z61wRODLgP+lZh2L
+auTGik+KD2XGw3o/4VzTcYgLdpPzCTJeX281O4lRt5cmL9/70lt5LkfaefXZT2Z
Kz8XvI1ewG+IPp0YgvY7h2UurbUC3Gg6lqyNyXHJ7r65mJ92nxceHLDEku617b4z
dDBf1iwlbem1DzWYF7TXpRcCgYEA0vO3O/Pf/BQtl0ohExH+acEpKv11r5Ge9yJ5
Rmr254tTTy/rD0+Y+5xhXEKyFvWOf1MrhW1wy+N5tUZ/5qBpq9yj6tMd1tek0Man
bnoqVApq1o4LuCyMuZg5QnKfSYbZsvC9s+tm46hAn25QoSKQiMvQzFkIlpI62XR/
1eDyBa0CgYEAgrbtprvvjYDubS35F01Es2cksNdQU8N8p8O4heRFFbpsyv+YCvPs
cJiZ0TFN+DhcDnFejSuhATP2lwEKBFW72iL4BH94GI/EDEad5bb4eLboja2vQ3pr
psuOT59tm8Jt7XrxHJjFT3J5zyefTT9GGLdk3BU+27pmaoTROgc+8bw=
-----END RSA PRIVATE KEY-----
`

	statsdCrt = `
-----BEGIN CERTIFICATE-----
MIIELjCCAhagAwIBAgIRAJ+nb32AaZrA1uJHoPiC7FUwDQYJKoZIhvcNAQELBQAw
GDEWMBQGA1UEAxMNbG9nZ3JlZ2F0b3JDQTAeFw0xNzAyMDYyMTA4MTlaFw0xOTAy
MDYyMTA4MTlaMBoxGDAWBgNVBAMTD3N0YXRzZC1pbmplY3RvcjCCASIwDQYJKoZI
hvcNAQEBBQADggEPADCCAQoCggEBANLhbjGORvMNQjRfATI4teW0DQsNAjfJm+DR
hoqaiTpliGYE7756c6bARCbTpUwMUxKe5ihbc3yfepqaHxP5fCoKns0Tjo0t9d+L
OSbrlIEvXhQp+ZwNFZyTfMDBDp8uMYLgIQHdUGmK6HZddZN5CQmE/m+4lNKSxzNQ
WyOt6aaBm1t7QvzaFHTzqPZCnXk3IyV5BTW7trxU9r2D7wgRc9g8HifoQvUQuAds
rUq/KTJ6P+NtVKLqIZVeh/EDrA1rPtv0AgZveYQAdh/EKyGbTqNZhUgwuFetfgRK
QuvGvrsNFyBjRd7E8Zl/S81foYJnt4llBJWyOsvKIW4mh9jSHWsCAwEAAaNxMG8w
DgYDVR0PAQH/BAQDAgO4MB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAd
BgNVHQ4EFgQU2u3+66vD0rtSV5c4EpRiGMyDsB8wHwYDVR0jBBgwFoAUboWohKPX
+GKYSC/kSGDaO4BqoLMwDQYJKoZIhvcNAQELBQADggIBAG/53IN+uDESJ1zYBJL+
45JZ39sMSxB+Wg+jjiQQwlejza1utt0f+/2K+lCpXEd0Ul8H1La1NlDpuvkKRoPy
jRtwWygqFpUw0KSHapIwk2stBkjOri2+mX63cIc0cUPUazDhV1ZHFS1goM3rnRKY
jWOqKdCOgTEO0kIVuBiCo3fT8L9B1nIWls4sKeDGOoXrmNk2mgFBMAmWiy7rQ1LU
knSWqk4Vw4dktlLXXTw512s8yuc8VftVZlqy2qzjJbJ3as8JAW5klqLCxJxk6FNT
JQ6Bm9LxAkL2mIdnPZbMk2cejHkjJ9fC8IowT6J2PzkPiocWNZ577kRyPwCLEZAR
vc0UNlH9yYrDHLrQgYWwfCS4obrhJ62KaRYrQSLNJO0KP+3kcE+e3yf+Zo9J4zC7
GFS/1d6KzC6VvRAwekfTpbKAWvxp5Lpxj1kx+utH/XBH26DTUXZodWwY+GSxUixc
zzape8XPjva4gokfbKoSDWWoAsVAZP2yZ55EBRp6PjkeT2UcP7FqTkO4ga8BOr3W
JDFXIwaO0XOS+WZuIlJq5wrnlkOoONGKzAJdWXgwKpM4F+5KlOoDaflxs6eiet/P
neCXSj/FKQX5hw7EjQTbVKXn6ROmRBPVLBpmF6kMvmIh+DatjE2eQi4JxaXBas23
Vk/h4DBjLuCPp9vKRGWhkdTs
-----END CERTIFICATE-----
`

	statsdKey = `
-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEA0uFuMY5G8w1CNF8BMji15bQNCw0CN8mb4NGGipqJOmWIZgTv
vnpzpsBEJtOlTAxTEp7mKFtzfJ96mpofE/l8KgqezROOjS3134s5JuuUgS9eFCn5
nA0VnJN8wMEOny4xguAhAd1QaYrodl11k3kJCYT+b7iU0pLHM1BbI63ppoGbW3tC
/NoUdPOo9kKdeTcjJXkFNbu2vFT2vYPvCBFz2DweJ+hC9RC4B2ytSr8pMno/421U
ouohlV6H8QOsDWs+2/QCBm95hAB2H8QrIZtOo1mFSDC4V61+BEpC68a+uw0XIGNF
3sTxmX9LzV+hgme3iWUElbI6y8ohbiaH2NIdawIDAQABAoIBAH6dVaExcM379vUk
2b4CqMw7N92buOowBYSNqP9NW/mQ/r1qV1wBf7DuHb1GNCgd+j7i4wP2LLf1tRJg
WSqQEAnaCJDPHjcMEmVe3TjOF4MdIppuvW0BuikhsLS29YWDobyXv5mz/NTxzzNK
WA7sjA2IKZCAvfJUqH/Zzm70u6X5cfim/ECh1CMSrFNQqIP6kOm3r5PUWC4D8GTA
gdT2NlQiv71x3iFBzY/OB1P2dutj3E7hMtV7OPwFvhGTsQuXsdVgNYty2k7Y0Fyu
Nm/D1ZPmowO+mn3BfeOiC1niD4KL8ld5CEIzu9/OzBwDIH4EX6fv9UhBbAXX8pYf
lScUpokCgYEA9xsm8EzJmxp0kX1ZQHPKHNSTe0GoBkeuVBQGVzphAnAZTYfUQUAI
Koueho1y8DOYlyIWU7o6ZxVems1AXAoUGv25cp+VML84lYL9N1Awee89WvwkxAH0
XEihOuGvcLUwDLkaYXATZSTgioRfWvnTS3iZ/hkHQIgMz5XdFAFBmb0CgYEA2nh+
tZ5jKKe49yayY8bTY5HJ9wwXM9BJF1WGvIhbEibhqMncik91HhX/VBQ5ZTeRlIsL
gtpfhV1nODqAp93cl69zgAsppQR9+cfL/LmITop75lwyYGvX4YGTgOf8CTt0uDLq
nF199ofLjG6/JfoluFuqT/TfXHTH9DszhqBxAkcCgYALo7TG1uccLjfVbpEYrxuT
FhRIVwRiH1g/z52o2DAfnEYk18QQusJntqHl9p22YMfMPqfMk9YSavhE1Gw2qabe
yprEom21mRxCNqRUyasu4y0BryTQMsNe4XDuxBiud2pm/wUWF+BiAEWvYKLZNzFT
ub+PL4Ce8omf8ZAzVAfSBQKBgEh/e+zhJp6zKdo6aTBbJoMAOjlVNc3n7JlltFSU
G0SmynOsqRbszzywqA5Kt9Ey44ibq7I8rT4ghMRQBamvIijj/DwdeDBekT92Yeb3
2pfVtM/5AG6m2vjmewBn+2dE57LIkrpY/Bf3cECl76C2phXLtXTbGdQgnMobaznd
vCK5AoGAMmULZ+IodtNb/oGhcU7ZTGtwwU4Tde1GaK2PzAjIppESrbOUiuLdDHYX
1nU/ME0BWYisoQJFMbi3GxIX5BTDS4pG2reYVR5zHbeITbhJR7yCjllYs/IJo4e9
1nqbfx6XnZ3TMrYZ6D4WDHVLU/Jc3o1lHMkJr5UZ4l+hvv1CCsg=
-----END RSA PRIVATE KEY-----
`

	caCert = `
-----BEGIN CERTIFICATE-----
MIIE8DCCAtigAwIBAgIBATANBgkqhkiG9w0BAQsFADAYMRYwFAYDVQQDEw1sb2dn
cmVnYXRvckNBMB4XDTE3MDIwNjIxMDgxNloXDTI3MDIwNjIxMDgxOFowGDEWMBQG
A1UEAxMNbG9nZ3JlZ2F0b3JDQTCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoC
ggIBAMLEEM5fWQo35JESEyLc4/WAU1Vpxw/qvkMWIC5LrJ03GTflW02YWlc4Fw93
51pc/Xp4PuXEHAOG4X6YTeCzr0XKTyeijnlAhbkuUj5/MXqv7FS99rR7HjdCuABb
PPzLsf8+q935C7RSoGKT3a0rmZT+KwLWTzPMH5Kb6o3XVVkhB5qTz0sjsl6x3MYO
zSb6cH9bZo1o1E4bOia5W/Y5IEv1T0QoE3+etuzZOur/MUUH7D52q4n/s9OO7ivJ
3ob+XfqS64s0ojAK2W+LbNE7WwKZmupnLJOQewtv3HHeqwxayv6XMc1V78y10W/B
NoOHbM2zb3QX2qKVEXfWKyrEKx2i6YsE4WJeavi0SeuWKInBpiBSj8Co388DXGsE
gPn6MV2Uo89dX7MzZBejKd3k3MlK8FN2G48zJoFV4Pr/887A6BuYwh7RhD9ibFUK
cKvbRNoc1AXaFW97YiHJa4h63aX344xktcDLFNvxc1EHnHLZuOcY+yP8nk17BxZ7
TkyTTn6oGJEHpqqQx8gF5lbI0f2K530UVTni7NPxMWDIBilfeYjSYB+dTA9hSpPU
1UEpsR7Uvg3XlmAJ/DKWCrgMtNsoEpzr2CM4GzCmARchiaAjzyrH9eyIqwyv+uIK
+gtpy5dH8o8MwYJCozmAiGr+NYselIhl1lsBGdK4ZaAcJN1JAgMBAAGjRTBDMA4G
A1UdDwEB/wQEAwIBBjASBgNVHRMBAf8ECDAGAQH/AgEAMB0GA1UdDgQWBBRuhaiE
o9f4YphIL+RIYNo7gGqgszANBgkqhkiG9w0BAQsFAAOCAgEAe2jxSYHjU+BUrqBx
st9oU2ctBaBhITXu+NKRYrHcifRlWPh5AbHjRpwibIV8LLZrISgxtlCyjPzkhv1L
Cx3Hu736p2oWKyRi4FBESUrwl+78/mZ2Ogs0fBsHKvV3gWyoj5fWHUVLcP9F4b4W
APuj90nGQ/+fXODxKf/JgfK74jF9ABWgGoEerMewfaMt7wVBru1c3Q4tfUD/GV9L
MidwKl0qN9Guk6WJv6IJNuCta/Ou7jzl/QeMWerZurkwNqtuxdJI0zaSANvFFYft
2A/FvQVRkzPuTkcn0N4AiNWb6dU5VHfH2hnx5A2fFzDTw2Y8NFAGP6u2uh+T08OQ
nUIitCEkL7bKYjEjAZdE1+Wc0MXdDHMTWwvGnXKxRe7zKL6kI55sSKJ7BpsU0pD9
Mm1pR79dFQTFexRPqUUaFJFcU+9vuJk3EBb6jSZaE+7aBk/h+QUoyNfJUgbey0yi
4Yv48zXaX5SrvNYpjM5a9ncvrgCbnRKx23O4vroapATTvEZPbbtZcrhyYsmpZq1U
3bmK9KW57yec9VBffXx04JwmFcZH+S2iN8+AEWnqATNjsK8SLuTT8FIL0/S/c/30
thFzlc3axP1wYLMMAjQX2L+QCSGyDn70SYWJu/Dv+ooIqDIiisJtQRqRDsoA/LmN
3OjgO6ui8tzIHyoWTVVpYEXigTc=
-----END CERTIFICATE-----
`
)