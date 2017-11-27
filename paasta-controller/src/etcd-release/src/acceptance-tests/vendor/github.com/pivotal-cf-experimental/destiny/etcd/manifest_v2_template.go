package etcd

const manifestV2 = `
director_uuid: REPLACE_ME
name: REPLACE_ME

stemcells:
- alias: default
  os: ubuntu-trusty
  version: latest

releases:
- name: etcd
  version: latest
- name: consul
  version: latest

instance_groups:
- name: consul
  instances: 1
  azs:
  - z1
  - z2
  jobs:
  - release: consul
    name: consul_agent
    consumes:
      consul: { from: consul_server }
    provides:
      consul: { as: consul_server }
  vm_type: default
  stemcell: default
  persistent_disk_type: default
  networks:
  - name: private
  properties:
    consul:
      agent:
        mode: server
        domain: cf.internal
      ca_cert: |+
        -----BEGIN CERTIFICATE-----
        MIIE5jCCAs6gAwIBAgIBATANBgkqhkiG9w0BAQsFADATMREwDwYDVQQDEwhjb25z
        dWxDQTAeFw0xNzAxMTIxOTU1MDVaFw0yNzAxMTIxOTU1MDZaMBMxETAPBgNVBAMT
        CGNvbnN1bENBMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAvnVQDLCe
        xBR6qUaGk1vdRyVGQnHsJnOtrshthaoshqOfsjwbpQsXBCRv5R2vIudebBN4T28A
        3WwgMBhLTiAiFU+7fWMPtIOcPStkc13stv9n09QaWu+hVtYrq5s0qS/2e3ZpcFkv
        wkuHyLzb/vYX06r65gHFMmHbh3GuzzlhlKpqKO++3bfe4GGWWB4AeXRqhMPB8ONP
        ap035q3zarmqp19hx55vIvbXszUGNGC56tWT5KxKk8feF73Fg1Nzd7+HIJSgPVoc
        1SkToiKbfjTyf6iOPiJZ5yrD3tD47IQY0DgFNlGB2lLtE38k1ktZeRF+eh9oMC1A
        XKHz73oljji9YcrKp6V8DkXggFctSspIHcbL5vuEMEeiUWWINvZRZaakWkqArVN7
        UFI4A35Uaitxkjh2ngdVpOYKCxZh7VdyxAnOMOrk6bBsfq42gbPanFxw52SYc/n+
        o0/RXsOo0MNcGa+bDGSY3qMR8BRWKdlPUippsQxSjqRKF3XxsvqUNb0Ub0qP7EWq
        XUUPmWR2VZFmse4tQZE6OlZO5pG5DoW8rcgn/+p0q46PJM/d5F/rxP4OaSePmL9Z
        sNUy6Ahv1Ez0ponaky3diZK5sytB6NOJhIUyvOaKrNHTR0ZP4W/Brk6XylG+wB6f
        d1xm0pA9/1RxpkrDGtIIBA5bEWw0avtDUR0CAwEAAaNFMEMwDgYDVR0PAQH/BAQD
        AgEGMBIGA1UdEwEB/wQIMAYBAf8CAQAwHQYDVR0OBBYEFMMEai/jNtmoqZ42xEC0
        wNmlA1TKMA0GCSqGSIb3DQEBCwUAA4ICAQCt1vjN6ecz3EH3rQDoPxnS8wlbdwlt
        QqsFp664pHXmJLFyeNEqTVTUSpRLdWkuyl0p6u9cLriovltM7IxwVX45Xuto4TG/
        g4Lha1OBDjmtamM+kRU+bMADSDfpR+89A8pTIJISIA69/xNT7L3xFuNmSHWgwlJt
        VQBe3XagAPk4azRAyMx7J5nswpltTR7UompiNceMQGHrihY33nesHHEb/YPKYP2K
        tjDFAJXyg9N61dyej+TykAtwriaLl5fBdHETsdBZlhMaHZFjqRJhAwsodz8etvoI
        BjfMu6u9Xbovfs7KLUMtFF8dbk3I8Wg0tty/uXAdZrEWwl8Wzd1KVQIUaJRkncif
        qJ7a7NWSXA1EGbx+WK16bJe5lwimbEpO5IkcgLwx+xQPJjt9pzIA522llll7kMNK
        lz0DO+XI+OX+l8d0fKBrYaY/Ei3rkxcAKJkOjTbgnKY2BkYMA3Ly6ZngT95zY9Qc
        aLszk3Xqmy+uNvJQZvOQ54mcp0+i4eg3ZO4YzuvIy7GxktYidn2/Hm2A+x9mzh90
        4vxnDL2r4mo7r2IWMJMd0vw+XJO870lEkRMZ9rp2wfgU2ijZn2UFnVRsl/doJ/vF
        lkuVVwDnmyBJzPfLEZrkl/z3k0lfxieilrLeO68/TJ4+MbNOdyfUFqjF1+0y2ebk
        Yn0I5jH3LNeorA==
        -----END CERTIFICATE-----
      agent_cert: |+
        -----BEGIN CERTIFICATE-----
        MIIEODCCAiCgAwIBAgIQLzVpihAzFtaw+0Nr81S4czANBgkqhkiG9w0BAQsFADAT
        MREwDwYDVQQDEwhjb25zdWxDQTAeFw0xNzAxMTIxOTU1MDdaFw0xOTAxMTIxOTU1
        MDdaMBcxFTATBgNVBAMTDGNvbnN1bCBhZ2VudDCCASIwDQYJKoZIhvcNAQEBBQAD
        ggEPADCCAQoCggEBANTMBAUp8pBiqWQGOAMwULjwyiSwkCfK8PggdO0PotzjiMrB
        EpY3rKaMZm7QrB9JKSFjisNY4L3eXcy9JgdIR53/Bnzbm1dTz283820L9XmuHMjE
        OFzicgHK8oBw2qL2H4gzRNy6YYgpc5QiXsOlO15nqj3j5B+7GVRV4On6cfT9FwQQ
        +GxcB5UKDLjL/7+rzYGVJnO+DokGtg7E3V9Cgd7Xum54oDKDvBfCVsSv9Gn38rSj
        iyZ79VZ8FJxc8sFqjRnboz/cIaBm8K0BcU4Oq8GscHHxpAUvcfLpITWLZoFhckCj
        0ApvcsoN9ZkxgbJHeowbsKqDH3Fy29e0a4kDgN0CAwEAAaOBgzCBgDAOBgNVHQ8B
        Af8EBAMCA7gwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMB0GA1UdDgQW
        BBTBXAohz/DCAzP6qd7rR25RIdJFCjAfBgNVHSMEGDAWgBTDBGov4zbZqKmeNsRA
        tMDZpQNUyjAPBgNVHREECDAGhwR/AAABMA0GCSqGSIb3DQEBCwUAA4ICAQAdAmKV
        H3J3jRewJizV8msbftbcqB/hdQRJhwcN+LNA14gGcTP2f34lO452MsdUUvRBz+Te
        D2HKUU0WobuIeCf0TvMreLP+kDBOrPGmS0BM0mA5OnExm5WNTLB2xujqhmbP4kOB
        Wm61XAXNANWrCkflm6axmSMUJd7RZIpZFyFTYeb2739IeEWY0s04GfLC1pmL++Tb
        OR7Auo+qvekgv+fGbaVo22qEpNdU9xxhbdMUMgT3WqQX49WyQBi5EDVkukXmoVRx
        bsjXtUpyY7i3vwZTvFeWLAyfwIpFv6KXtYZ9NLJ3/2pzahBOZX8S+PnfV3rnJTjk
        P9z96ZR+9hNckXz0Wyve47+ffliR2YU1ouKjNWumZh/d1x1oBlV+ZWKgBUVL2vmw
        jkWw2W6aVahjfukyjyiFgI6K7KDcFvl5ce/3ub8k1/jTIjs/+OiaM2uVesUEkd2Z
        u4dk8Cw6bxr3zB8L+wPX1IIDhi+aOQw8gWDGezmHnC+FruEQvrwbOVvn0pwe93SW
        CNLmvj+2Rbclk9nqB1KBwWQur+4OM04mfT15QptV8adTnaC0wJQKD7ZsluwHZYQF
        ALnJk09TXg+eZtMJ9zD4nPvssEOal3VzZ7tPPzbuartWSZMOlDS5HziB1OQ4Gh8X
        s9pKNZhW+cRPukmjNHHjU/LF6NmkSUmqmiI8Tg==
        -----END CERTIFICATE-----
      agent_key: |+
        -----BEGIN RSA PRIVATE KEY-----
        MIIEpAIBAAKCAQEA1MwEBSnykGKpZAY4AzBQuPDKJLCQJ8rw+CB07Q+i3OOIysES
        ljespoxmbtCsH0kpIWOKw1jgvd5dzL0mB0hHnf8GfNubV1PPbzfzbQv1ea4cyMQ4
        XOJyAcrygHDaovYfiDNE3LphiClzlCJew6U7XmeqPePkH7sZVFXg6fpx9P0XBBD4
        bFwHlQoMuMv/v6vNgZUmc74OiQa2DsTdX0KB3te6bnigMoO8F8JWxK/0affytKOL
        Jnv1VnwUnFzywWqNGdujP9whoGbwrQFxTg6rwaxwcfGkBS9x8ukhNYtmgWFyQKPQ
        Cm9yyg31mTGBskd6jBuwqoMfcXLb17RriQOA3QIDAQABAoIBAQCO9TCON4wZq+6Y
        oATpP4A7fqiO1X9C/He+ei+TQznqo4G2lNbjzCtVCGWYdN/tdL0JDVKfwgnaBJWH
        glsV8V0Lq9Sz9OT7Wfa1hSUoUSxsvqffyNMEs6xbv/gCic6YRDkSyz6r+xqi2xYm
        oqB/V3X3CjW4tmz/VDbEDZ24EuST7GnrumU4RHbGW1aqOpVhRX/3zIQ+f9YQGFMZ
        /BkusrSfVUqlV2VHGZkxsyA8+Xd8Jyi/TAksnxx/2HoXjcjGr5XUrvdM/XVdlAtr
        52TmUn0Fpyu7oYf4oKFknVRzXmxPy2WeCyaiGAq+m/F7JAF/CIqIusA9ASR+B0R4
        1Rw79H+BAoGBAO4Lxuo/5Q7j6cDRAMFQIVazrc/0ykwwKytAqjZoavzTL4zANt0G
        1ezu9tK7uzPXgtlFNiZ3gIpQDWAfPRE4gLXjmwB3TIh1EiBOwTgtfzGoUCp3wWmR
        ARHb933ik+xQnG4u6P6iHrzWAYZUprvumv3rDEK/C2bmAMsqj9WWJQHVAoGBAOTY
        ufeNL0XYZYGTXEJj8RIi7owuUQUq4tOcu3BoexLKbjs/faLlucMXstPs5ae5JKsP
        5weOEgqgiKZNHvLh619opdTJyggjcEyK48SqLmEAwJ+M8EV3LYyw6zZtKWQDR15N
        OBTNPTc6jQzqUOE8z/4ZP/CVkW95HqXOOtBptn7pAoGBAJsuHD8q5gTd+M1UsnxS
        41jlCyLs/k/KeunYXt3XFh+5AF9uEpXl1eF+KnNYJIJ4NHm1D8bl0mrYItANrT6j
        qexo8uvL2Z1/TBC5pmYb6rYRdikpJnHOMHdXATEUWsAMEN4XQJZ2Uzlg/V93obYT
        pwBukPCWIDW1LMFE/r0LAxb9AoGAO98fuE5twb49wErHZm8zUOVmt7IebFWuBmMI
        /v22xVHEySdxPT8Q/KOkm6Fs7BaaK077yJQ40CLz3V5r7GuC4vFEAYnRm5N5++yS
        bo9/ls1Vl+iNq/7kIdzfjNu+anYZI+jb9UVE8MAWyvw6sNLyL653dgALjriHdiWg
        aYpevpECgYA5Wh1o5TCjxBYaoPNMy432OwGiuUuP4pU46OBH5b9G+aHxPmFIQ1/b
        axQ9CBFypBMp1GVYym8k0xhV5NU6UO8sUwMz4+g3jpECLxO6z2JS5IdRsuLZWLxf
        5XnBawt8jmo/1wrttqr1EFuhNWOw1ypIKnTYk0Zfe42mwZuUvWxYpg==
        -----END RSA PRIVATE KEY-----
      server_cert: |+
        -----BEGIN CERTIFICATE-----
        MIIEMDCCAhigAwIBAgIRANoqBUcDT6LcsutWb/sz3fYwDQYJKoZIhvcNAQELBQAw
        EzERMA8GA1UEAxMIY29uc3VsQ0EwHhcNMTcwMTEyMTk1NTA2WhcNMTkwMTEyMTk1
        NTA2WjAhMR8wHQYDVQQDExZzZXJ2ZXIuZGMxLmNmLmludGVybmFsMIIBIjANBgkq
        hkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0pS/oJQJm0WKU46sYGkJTIvptr3dSM53
        qCmkampDRc04PrfWHFOsl7mTCxUDoYtNalzk5P7iAS+NYZjeL3ceB+K7XoP35l2G
        Y858A5uNjQbjzsYpI1+vZvLi783L8NpnMecnBteZCn0X2pqEWP12detiyqiHdTSs
        e/AWnXzu1nt4VCEKU7UsCDF2zUVYtlZzYqY5MFoDFm35egNJQ0Gzw4hvRMZGObiK
        +3El+CATbgSKX6jIhiyiqeVEIc2zw/LNtHs1k3+vSMB0NONTG+ledat8zYVP4nQK
        3Mw5CwZ9ATMgSftKpC9qvJIcLOiGaoFqXFqmdprreYTNAEB47le5kQIDAQABo3Ew
        bzAOBgNVHQ8BAf8EBAMCA7gwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMC
        MB0GA1UdDgQWBBRgkvQ3qtsnyyjF+NP9+DiM2pL6EzAfBgNVHSMEGDAWgBTDBGov
        4zbZqKmeNsRAtMDZpQNUyjANBgkqhkiG9w0BAQsFAAOCAgEAuMuzWC6kQQNyWI3P
        EsCcON8GZ+dzmEfn864zwWTtxtiMi2Xey7RHd8lS7t8kIinWYLaVuaKK4a2uvB+T
        YPtLrpCu7S3E5MM/ADlW1qxZRM+nez1Ud5kHL6zg4HFOgHMvbF8GMq5+Gf1MDeS8
        ritAf0DnaKmJIKXbYpWt2BK0GaNQLxk7A6nyFCx8Vzq8Xq/RePCy2QugjauPlrMG
        tDrusUVL99aZEi0ZFlmNxplpiDlNI0exH2Q0vUvlwvo3J68GdR4GQIT7QQwJBqRR
        FT67NmkyMEn1vq6Y2aR+hPZlzlvJJzaP/PUA5x2qeK6vKG8CN3i91b18pAoD5sAi
        LuFPiajcu3ZipN+TfKW2zeGXZ1FeN6uxIvZdQU/++6VmBAii3TRtQoJ1NnHGbW1Z
        nNz69Z7oTpGa/0ErEeNIRo5drL29aMMBsYk/x94+z9WydgCbxkcCCR9fQKrrUUYU
        jPPALsXSnnaYKcqEnjEb1lSogCOLLihco2zOoeO7pCtIGwcLzlxU9Fs3t/P+TW90
        3az0e40QNFzz1Dxu7iGBdKT84icyR3vIVZV/a0DHyP2BMibGaA5EIlvlpAwCxz8Q
        rxBY+Mdvwdux9fncq6j8xr3nXO2s+SxW7cOl23Avq59Snpcr73ipt3+fJSMOPJdc
        Wm4+9+jefDizyfRIwml1Ok5UvDE=
        -----END CERTIFICATE-----
      server_key: |+
        -----BEGIN RSA PRIVATE KEY-----
        MIIEpQIBAAKCAQEA0pS/oJQJm0WKU46sYGkJTIvptr3dSM53qCmkampDRc04PrfW
        HFOsl7mTCxUDoYtNalzk5P7iAS+NYZjeL3ceB+K7XoP35l2GY858A5uNjQbjzsYp
        I1+vZvLi783L8NpnMecnBteZCn0X2pqEWP12detiyqiHdTSse/AWnXzu1nt4VCEK
        U7UsCDF2zUVYtlZzYqY5MFoDFm35egNJQ0Gzw4hvRMZGObiK+3El+CATbgSKX6jI
        hiyiqeVEIc2zw/LNtHs1k3+vSMB0NONTG+ledat8zYVP4nQK3Mw5CwZ9ATMgSftK
        pC9qvJIcLOiGaoFqXFqmdprreYTNAEB47le5kQIDAQABAoIBAQCdsuWa5KIZFMfV
        cVgnzyE2oOTChIdN+cjkN2M4iiGdCWWgml2O0x7CdSfoObGBbefoym5kC3jG+IyB
        VVC27RahQyucSWoBq3J0FfMLZJdp0IoTlJTEN+kMSMKoYU7kLTrwxTGVzyl+EFYn
        0GVim1X2UvOl3vWqUWsGWbMl96SJG4ttbywpTHhXyaZvijqiU24OgCKa3RjBMb/b
        mpSt5CZyU2UK7QnASxgA2DEO6scwvK4WZVINAy3XcpQy+KjHNPPl9HPTcQvjNo2k
        arCp+FJqi/HCVBk4Ww7rVUaLMi2j40IYrl2AW9GkAULEAFKD7KoIsjBNz1+UeT9/
        OG2UD6b9AoGBAPfnG8cZMY03HAvk8scQkCyopLlIM7o/ksthcnMT4nRChv9qAy8o
        Eg2VMiAF78tRB03QeBAJMh7dL+R8p4icQv3nSOk/x9Q/ve7oeb3pU5VT9+SEkhYA
        1bwxBfMMf+mjBCjYsCGpEa7AtZ5A1jL/7xK3DREHd5eMWVuOnP4as+x3AoGBANl1
        kQuUuml1yWGzPCo7DEd/ucB8tH9rOEAUQ/M9ykprMAUTEblll/IkDELGgTlpYR7F
        GKwVJrt5ClXAp3tSWGPBkr6fUMS8L2Piaiv7CK737eYLfAvWLEBFxzvbYD+uRuM5
        sBHFU4XE0Qcz6tu4u9IjZTpsiW+P4YliHa2ORnQ3AoGBAPL1d83rrRq/lic6HY5n
        d0WtirNkRf4VbGMTgD20kU5sHS6Z0cEXvom9XUDxUJCtO0FSPTlKKesB0HxYh0Fm
        FGoPkO+46LnmNtm80gQEdzx07RDztNEHxHIKgdAwwfRTJjJ6HDUBJClnCRiuZr/Z
        AZAQAyhbbyQCE1meLdMEjK4FAoGAD8OtGyjSBrkqOzHyJ6GWN0y0G5cuwpn0Pvj5
        IBYXpyN0HLoQK9+Ij147oU+gqJfSGZfyPO9fmnGg5SyNN6x1ie3LhJQqF8kIqnYM
        elm9fGmuzmGAwZ7qIFKuqdEyfgtVSj2xXOhwMJ9fA+WonfsbapV0TjL2F6dXk00Q
        l7dbtisCgYEAljP9kQY+g4B7DZ3H/ljy2shkcDssu2MqDFOQ3dM+MCv8vdjUmEUG
        3ICL7s3crk94rPTgWwfGf4TWHSpokPJDjANil6H4r0TfXiokL4E2PSy924yUwX6R
        yZyRpy84+Dn7BIcSHK/fJHl6TMiyObmBrYglUHDL7wmbq3y37LVTaNE=
        -----END RSA PRIVATE KEY-----
      encrypt_keys:
      - Atzo3VBv+YVDzQAzlQRPRA==
      require_ssl: true
- name: etcd
  instances: 3
  azs:
  - z1
  - z2
  jobs:
  - release: consul
    name: consul_agent
    consumes:
      consul: { from: consul_server }
  - release: etcd
    name: etcd
  vm_type: default
  stemcell: default
  persistent_disk_type: default
  networks:
  - name: private
  properties:
    consul:
      agent:
        services:
          etcd: {}
- name: testconsumer
  instances: 1
  azs:
  - z1
  - z2
  jobs:
  - release: consul
    name: consul_agent
    consumes:
      consul: { from: consul_server }
  - release: etcd
    name: etcd_testconsumer
  vm_type: default
  stemcell: default
  persistent_disk_type: default
  networks:
  - name: private

update:
  canary_watch_time: 1000-180000
  max_in_flight: 1
  serial: true
  update_watch_time: 1000-180000
  canaries: 1

properties:
  etcd_testconsumer:
    etcd:
      machines:
      - etcd.service.cf.internal
      require_ssl: true
      ca_cert: |+
        -----BEGIN CERTIFICATE-----
        MIIFATCCAuugAwIBAgIBATALBgkqhkiG9w0BAQswEjEQMA4GA1UEAxMHZGllZ29D
        QTAeFw0xNTA3MTYxMzI0MTJaFw0yNTA3MTYxMzI0MTZaMBIxEDAOBgNVBAMTB2Rp
        ZWdvQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCsrzEJ5hAQkdkT
        l6z4ffiYvq4RSxKXkeZWTHv5b1w6FSnGCVoQL0ilKyQTGzn001TsZBhqJRmhKvLs
        /4RC8a10KK8hmVhoV4MX690Abd47GbRQR6EPdcd4URHqr0NeeUIPviZGk1EYpFaM
        T81eVq15Q+VrakVfGMjPIPfGqtXV14fs9jvkzVAdTysM8AtZtfwQC3ohVfkL7wA2
        /Xs2YYQdLI1dKNnYdDxaDYmbjjCmxTMlkrloFBLmNveEEpy9Vnw3mcGyuAvq8PEr
        Uua58czKsb81bONp7hzjK8I7BvpvneGTPXg7zzuVRRTwRhZSOoNcqE3/+EjJd5/W
        ONtAYX66xN9apYGHcSmWDFxH6RBwLzJzJOo/FJ0AD5BkQBjJ4x5ZX+5X05oAegj1
        wUYx32q2IrDIJzNF+CltrhY+bhJFmEqy72nomQPowSvuydlJMOYH5ATE8Lww0XzA
        FmhityWvbmrgneSYdg9RvzbqLGTbuEBJ2D+X5WGtAlyvKRehoSJcOr0h9iRCnZIW
        hu9YV6aBsVJHHyc1C4d4cpOx0U5QMXy05Z5wdSQra8n8pG7SC2K9V8HbOidr+4wI
        ZWHwAIgyA0bVvHdGrGeeWeyW/XXD4YGyCAnT4DXWhTLPgxu4gg4rf7nnyHKcAqYp
        DgHKMZOYTnbjCMcXyoYIJ8dR/RvYOQIDAQABo2YwZDAOBgNVHQ8BAf8EBAMCAAYw
        EgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQUAU5pu7rUL87RDYhHRL+YYfgc
        /4YwHwYDVR0jBBgwFoAUAU5pu7rUL87RDYhHRL+YYfgc/4YwCwYJKoZIhvcNAQEL
        A4ICAQChLHQM6f769dt9L6MmjOLcYdmmMuyxY8iqdnJIa43MBxKjxzmt6xSIPMBU
        BWFui5gScKPXiA9Nri2Tyzm5zjcQtoJUFcXA8RGgK4aVQ1QCuY4OyiR126WfZiiJ
        J0btSmUXGIme25KEQ2PSiYmwPrLTFG3G+0ylUq6b/rPzHfkFOZXX4U9qLvqY9AnO
        NuYxLT40xDwlL6drcvicEfZ+vV0SABf4HAH+wphRyHR4fkwOBrrieBXvpRUlGeRw
        ZtDVeX8v28WZqoYXV/36JrGbhxSkqBXQk5gdrOUDXebaeQPRvarWCd2zSGmyADei
        npMRDEovA7AlyxX//vBx9MKV3L3NhoL66nBgOwm23DZJLIwCM5AIBvyZMfMpB4sM
        d2nUiXF+5WRFG1bjHuEmU0HvZGXFFzJaiJrnlvzDhJB32DQ5LgEeN+9X42x3DXUZ
        +dR5Qqu0wgQGpdjC9sNsgMBcqVqmc8rWfRxHSusHff7tFs8gpzNRxH6Rimws9M0d
        RFWLAS0T7YSB6deM41Efz7T4Gq+QLm7sv73pDhuIky+AZlWkAr9Wu/+RpNvcQfum
        r5EejEQP82achV3em5+macfNfEIILruStanw9D+kR1GYlE07wMTTmkZ39x3HMicf
        r4ERoMvnaSaiGVHIiCi9ZsoNLlf6TBNNfaqpc8jDZa2/o/nM+Q==
        -----END CERTIFICATE-----
      client_cert: |+
        -----BEGIN CERTIFICATE-----
        MIIEJjCCAhCgAwIBAgIRANL9v1f/WA6jL8gV3RY4HE8wCwYJKoZIhvcNAQELMBIx
        EDAOBgNVBAMTB2RpZWdvQ0EwHhcNMTUwNzE2MTMyNDE3WhcNMTcwNzE2MTMyNDE4
        WjAcMRowGAYDVQQDExFkaWVnbyBldGNkIGNsaWVudDCCASIwDQYJKoZIhvcNAQEB
        BQADggEPADCCAQoCggEBAK954XExQ8L+SvxD6Z1EodPDjZj5uXo1lZbvKBepQVJp
        HIKX6HWSXfWCjrsbVTh62jenISmcftt+7jl428ny96W4QDTDIVGzCnv4ISgQeZsn
        jz0u+KIw7ideAEEM2bXmDkyZlaG+m4LLvI0oIDwGIUaHfCZVmwP2vf03kwEOZFIV
        Qe59u9ITMuSWKyo8qNtgYgdBywlQ3c6vmD4tUZv/9s0r2vnd5H74Zqz5AJYEMy4I
        5f0+FLfDFIk3BVB3HuyY3m8h/N6AQQE6f0PmtRmaYbWE7Ys7tO9B7m5yIoBoB/Mq
        KG0/rvcZXadKM1sOLLkJV8j9nK2dY7tyJ5sh3ViiqWsCAwEAAaNxMG8wDgYDVR0P
        AQH/BAQDAgC4MB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAdBgNVHQ4E
        FgQUxis5Sa2Gh2U232vDW7un7G7orRcwHwYDVR0jBBgwFoAUAU5pu7rUL87RDYhH
        RL+YYfgc/4YwCwYJKoZIhvcNAQELA4ICAQCZRvmGJ7+XSCar7yebOhfyGHLFs5DN
        s826z3Nq526JiJTfP68LyLOTfD9PGX1e5Cy+sgfLsKZFKz3eha2DD05FKzoXegOj
        emF40MVTRS5Ik/8TKDQSJlfZPlDnYlnsdpLqGc4doB56bw1Czx88HOsCESKdDzh7
        yBc0olYtm+RX1qqJ+QIx8r/QTBuOIHg6K7+nkrX8pol6SII6vbdmbLye3et0TYY0
        93uaWnjem4lK7orRA0XkhlvqSTes8mndoIkKz1Uz6iT9dZagL377AAZnRSrsZVfw
        59wGMD/kyUlN3Q2Nxuq6zP2JRYq0zdU/T4m2xvyovaUZ+/L6jXfz6wrKPawrQcmI
        T2qCxpmekVqx7vXcOaE+U/9GOFM1FHy+ADKPNsU17kj6M1+I50gD7RNtIKEfaz/w
        ObtP0atvKlsha6a+3nuj9SzyK8kwIcvaBNBO02kDzBiamA6Ip5lFu39p4l6q+ng6
        3qjvF07GBxN9H3l3YF8xkcJxgukNhc4cuM42NGTt5gC3AWyej0Aqc4qu4tGhspcz
        O+j24BlEnVaWgikPDepA1Fpz3Qn3ewvA0DITEdAa9YU1m27k4pt9SnbAKmGyhXSV
        ObqPo5mxg8b8GIrRBTIuPDV4mKVx2eV+PczsPqg0UZJStHkz9+vX7X1pI6yYIGxF
        w7GxK8RhEJCvLw==
        -----END CERTIFICATE-----
      client_key: |+
        -----BEGIN RSA PRIVATE KEY-----
        MIIEowIBAAKCAQEAr3nhcTFDwv5K/EPpnUSh08ONmPm5ejWVlu8oF6lBUmkcgpfo
        dZJd9YKOuxtVOHraN6chKZx+237uOXjbyfL3pbhANMMhUbMKe/ghKBB5myePPS74
        ojDuJ14AQQzZteYOTJmVob6bgsu8jSggPAYhRod8JlWbA/a9/TeTAQ5kUhVB7n27
        0hMy5JYrKjyo22BiB0HLCVDdzq+YPi1Rm//2zSva+d3kfvhmrPkAlgQzLgjl/T4U
        t8MUiTcFUHce7JjebyH83oBBATp/Q+a1GZphtYTtizu070HubnIigGgH8yoobT+u
        9xldp0ozWw4suQlXyP2crZ1ju3InmyHdWKKpawIDAQABAoIBAFwO5xUJMXGFEzXR
        MyhMr1F3kDunF4VjwzzR7wiqxRhFCK4Cn/O+fAinG9ZRep4M5Zq41Y8NCQiCSNxh
        6XzDOOT6CsUjccF42pE7Fbn9Gq8pS95fXBVK8kY47I0z/quNLAdHs9aNNuyhkiPD
        31VeKerkfV9nHdIwim/jzf2J3Vup6GuCS1eE/J49JfaQxPBmxJyhlcXOfOOSA3nV
        RtEvFhHtfha1AWsU8m6hzPM34Tjxyr4OHcXu0oZ5OX+S8l+fF/6Pr1d3d1TKGk0M
        vlzYCWxEQGSe3HZ6CZb4u6ykIn9Feq7jHaCnC1LEH6OxkXTsv5D5GTTKRRzbLS0S
        eR8XCFECgYEA4uZtglkNd3I06mwoomNVZsd43Tf52yBcYgpdoMRKpRjYBwFSCsb3
        MGc9aUgA7oCGD5z4Ybt8fUXCAOXxo7McrUEW9p16SQr8nOGRa+jdsTue4DX6NvtB
        F1g636mc/FgYgfeoK4oiG6x7+N/ZDZjISuygwS6NThBtn56Vnr8UmPUCgYEAxfsa
        OarvsRaLsTSQhaI7lG2AF3Gsw/jswBWEL5xSV0BbCQ7Bm57ZqexK6PojOOCn55tP
        izHpGTobakxCL96IH4GWOyPcFnUyM4T2iRuaYJiIbJo5VpmMaveFpSwfeMPTwu3f
        QcF8LfeIl7u5M2PBfGKqEY2pEN1VfwFNA4N4PN8CgYAPfyVjjal5yvcKO7DaxmYC
        ywTaNwR9jsxAdezHGiDu/a9jaxerXMNtLt/m3OATafu9/T6JjkCGXclOPmYuhAEl
        ZBipZz/+1R1DqbRA5nqdrDDBp24bazWa3o/GztLF+U5TMhLuRlTmBvXAnak5YIHt
        fBPOndtQxZZ3HGGjofFKMQKBgA44KbsIluyOJPxWPScL7uGLN873QCRXJZHqObM9
        tABGRAOThr5Jm3KD4SF4jb0RDZ4p3n2t2QMR1FQ/I+XSQs6YfRTET5NhWXivzREt
        5VmYuvup3AJnRtmL65JgZ+ZBkl0Gvqk3X1bh13KmbffN62CmqXZXSVRHwVM84a4l
        7CXbAoGBALyCEhESCY4p6zcUej9M/7eGbqFIo/HXpfe0m/A93J6LwwRWSuDNqk2O
        r5qBJiAoVtuF9IlzLXKnkHo4oKS3EU0Fpe3zFkn1kluzPSWgfoEN7+QdXv3ppnYO
        1QEHVOsm4YyocfmEAdeBPW125nh12k7nZ79YUYCVqhF3jVFn4aH/
        -----END RSA PRIVATE KEY-----
  etcd:
    peer_require_ssl: true
    require_ssl: true
    heartbeat_interval_in_milliseconds: 50
    advertise_urls_dns_suffix: etcd.service.cf.internal
    cluster:
    - instances: 3
      name: etcd
    ca_cert: |+
      -----BEGIN CERTIFICATE-----
      MIIFATCCAuugAwIBAgIBATALBgkqhkiG9w0BAQswEjEQMA4GA1UEAxMHZGllZ29D
      QTAeFw0xNTA3MTYxMzI0MTJaFw0yNTA3MTYxMzI0MTZaMBIxEDAOBgNVBAMTB2Rp
      ZWdvQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCsrzEJ5hAQkdkT
      l6z4ffiYvq4RSxKXkeZWTHv5b1w6FSnGCVoQL0ilKyQTGzn001TsZBhqJRmhKvLs
      /4RC8a10KK8hmVhoV4MX690Abd47GbRQR6EPdcd4URHqr0NeeUIPviZGk1EYpFaM
      T81eVq15Q+VrakVfGMjPIPfGqtXV14fs9jvkzVAdTysM8AtZtfwQC3ohVfkL7wA2
      /Xs2YYQdLI1dKNnYdDxaDYmbjjCmxTMlkrloFBLmNveEEpy9Vnw3mcGyuAvq8PEr
      Uua58czKsb81bONp7hzjK8I7BvpvneGTPXg7zzuVRRTwRhZSOoNcqE3/+EjJd5/W
      ONtAYX66xN9apYGHcSmWDFxH6RBwLzJzJOo/FJ0AD5BkQBjJ4x5ZX+5X05oAegj1
      wUYx32q2IrDIJzNF+CltrhY+bhJFmEqy72nomQPowSvuydlJMOYH5ATE8Lww0XzA
      FmhityWvbmrgneSYdg9RvzbqLGTbuEBJ2D+X5WGtAlyvKRehoSJcOr0h9iRCnZIW
      hu9YV6aBsVJHHyc1C4d4cpOx0U5QMXy05Z5wdSQra8n8pG7SC2K9V8HbOidr+4wI
      ZWHwAIgyA0bVvHdGrGeeWeyW/XXD4YGyCAnT4DXWhTLPgxu4gg4rf7nnyHKcAqYp
      DgHKMZOYTnbjCMcXyoYIJ8dR/RvYOQIDAQABo2YwZDAOBgNVHQ8BAf8EBAMCAAYw
      EgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQUAU5pu7rUL87RDYhHRL+YYfgc
      /4YwHwYDVR0jBBgwFoAUAU5pu7rUL87RDYhHRL+YYfgc/4YwCwYJKoZIhvcNAQEL
      A4ICAQChLHQM6f769dt9L6MmjOLcYdmmMuyxY8iqdnJIa43MBxKjxzmt6xSIPMBU
      BWFui5gScKPXiA9Nri2Tyzm5zjcQtoJUFcXA8RGgK4aVQ1QCuY4OyiR126WfZiiJ
      J0btSmUXGIme25KEQ2PSiYmwPrLTFG3G+0ylUq6b/rPzHfkFOZXX4U9qLvqY9AnO
      NuYxLT40xDwlL6drcvicEfZ+vV0SABf4HAH+wphRyHR4fkwOBrrieBXvpRUlGeRw
      ZtDVeX8v28WZqoYXV/36JrGbhxSkqBXQk5gdrOUDXebaeQPRvarWCd2zSGmyADei
      npMRDEovA7AlyxX//vBx9MKV3L3NhoL66nBgOwm23DZJLIwCM5AIBvyZMfMpB4sM
      d2nUiXF+5WRFG1bjHuEmU0HvZGXFFzJaiJrnlvzDhJB32DQ5LgEeN+9X42x3DXUZ
      +dR5Qqu0wgQGpdjC9sNsgMBcqVqmc8rWfRxHSusHff7tFs8gpzNRxH6Rimws9M0d
      RFWLAS0T7YSB6deM41Efz7T4Gq+QLm7sv73pDhuIky+AZlWkAr9Wu/+RpNvcQfum
      r5EejEQP82achV3em5+macfNfEIILruStanw9D+kR1GYlE07wMTTmkZ39x3HMicf
      r4ERoMvnaSaiGVHIiCi9ZsoNLlf6TBNNfaqpc8jDZa2/o/nM+Q==
      -----END CERTIFICATE-----
    client_cert: |+
      -----BEGIN CERTIFICATE-----
      MIIEJjCCAhCgAwIBAgIRANL9v1f/WA6jL8gV3RY4HE8wCwYJKoZIhvcNAQELMBIx
      EDAOBgNVBAMTB2RpZWdvQ0EwHhcNMTUwNzE2MTMyNDE3WhcNMTcwNzE2MTMyNDE4
      WjAcMRowGAYDVQQDExFkaWVnbyBldGNkIGNsaWVudDCCASIwDQYJKoZIhvcNAQEB
      BQADggEPADCCAQoCggEBAK954XExQ8L+SvxD6Z1EodPDjZj5uXo1lZbvKBepQVJp
      HIKX6HWSXfWCjrsbVTh62jenISmcftt+7jl428ny96W4QDTDIVGzCnv4ISgQeZsn
      jz0u+KIw7ideAEEM2bXmDkyZlaG+m4LLvI0oIDwGIUaHfCZVmwP2vf03kwEOZFIV
      Qe59u9ITMuSWKyo8qNtgYgdBywlQ3c6vmD4tUZv/9s0r2vnd5H74Zqz5AJYEMy4I
      5f0+FLfDFIk3BVB3HuyY3m8h/N6AQQE6f0PmtRmaYbWE7Ys7tO9B7m5yIoBoB/Mq
      KG0/rvcZXadKM1sOLLkJV8j9nK2dY7tyJ5sh3ViiqWsCAwEAAaNxMG8wDgYDVR0P
      AQH/BAQDAgC4MB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAdBgNVHQ4E
      FgQUxis5Sa2Gh2U232vDW7un7G7orRcwHwYDVR0jBBgwFoAUAU5pu7rUL87RDYhH
      RL+YYfgc/4YwCwYJKoZIhvcNAQELA4ICAQCZRvmGJ7+XSCar7yebOhfyGHLFs5DN
      s826z3Nq526JiJTfP68LyLOTfD9PGX1e5Cy+sgfLsKZFKz3eha2DD05FKzoXegOj
      emF40MVTRS5Ik/8TKDQSJlfZPlDnYlnsdpLqGc4doB56bw1Czx88HOsCESKdDzh7
      yBc0olYtm+RX1qqJ+QIx8r/QTBuOIHg6K7+nkrX8pol6SII6vbdmbLye3et0TYY0
      93uaWnjem4lK7orRA0XkhlvqSTes8mndoIkKz1Uz6iT9dZagL377AAZnRSrsZVfw
      59wGMD/kyUlN3Q2Nxuq6zP2JRYq0zdU/T4m2xvyovaUZ+/L6jXfz6wrKPawrQcmI
      T2qCxpmekVqx7vXcOaE+U/9GOFM1FHy+ADKPNsU17kj6M1+I50gD7RNtIKEfaz/w
      ObtP0atvKlsha6a+3nuj9SzyK8kwIcvaBNBO02kDzBiamA6Ip5lFu39p4l6q+ng6
      3qjvF07GBxN9H3l3YF8xkcJxgukNhc4cuM42NGTt5gC3AWyej0Aqc4qu4tGhspcz
      O+j24BlEnVaWgikPDepA1Fpz3Qn3ewvA0DITEdAa9YU1m27k4pt9SnbAKmGyhXSV
      ObqPo5mxg8b8GIrRBTIuPDV4mKVx2eV+PczsPqg0UZJStHkz9+vX7X1pI6yYIGxF
      w7GxK8RhEJCvLw==
      -----END CERTIFICATE-----
    client_key: |+
      -----BEGIN RSA PRIVATE KEY-----
      MIIEowIBAAKCAQEAr3nhcTFDwv5K/EPpnUSh08ONmPm5ejWVlu8oF6lBUmkcgpfo
      dZJd9YKOuxtVOHraN6chKZx+237uOXjbyfL3pbhANMMhUbMKe/ghKBB5myePPS74
      ojDuJ14AQQzZteYOTJmVob6bgsu8jSggPAYhRod8JlWbA/a9/TeTAQ5kUhVB7n27
      0hMy5JYrKjyo22BiB0HLCVDdzq+YPi1Rm//2zSva+d3kfvhmrPkAlgQzLgjl/T4U
      t8MUiTcFUHce7JjebyH83oBBATp/Q+a1GZphtYTtizu070HubnIigGgH8yoobT+u
      9xldp0ozWw4suQlXyP2crZ1ju3InmyHdWKKpawIDAQABAoIBAFwO5xUJMXGFEzXR
      MyhMr1F3kDunF4VjwzzR7wiqxRhFCK4Cn/O+fAinG9ZRep4M5Zq41Y8NCQiCSNxh
      6XzDOOT6CsUjccF42pE7Fbn9Gq8pS95fXBVK8kY47I0z/quNLAdHs9aNNuyhkiPD
      31VeKerkfV9nHdIwim/jzf2J3Vup6GuCS1eE/J49JfaQxPBmxJyhlcXOfOOSA3nV
      RtEvFhHtfha1AWsU8m6hzPM34Tjxyr4OHcXu0oZ5OX+S8l+fF/6Pr1d3d1TKGk0M
      vlzYCWxEQGSe3HZ6CZb4u6ykIn9Feq7jHaCnC1LEH6OxkXTsv5D5GTTKRRzbLS0S
      eR8XCFECgYEA4uZtglkNd3I06mwoomNVZsd43Tf52yBcYgpdoMRKpRjYBwFSCsb3
      MGc9aUgA7oCGD5z4Ybt8fUXCAOXxo7McrUEW9p16SQr8nOGRa+jdsTue4DX6NvtB
      F1g636mc/FgYgfeoK4oiG6x7+N/ZDZjISuygwS6NThBtn56Vnr8UmPUCgYEAxfsa
      OarvsRaLsTSQhaI7lG2AF3Gsw/jswBWEL5xSV0BbCQ7Bm57ZqexK6PojOOCn55tP
      izHpGTobakxCL96IH4GWOyPcFnUyM4T2iRuaYJiIbJo5VpmMaveFpSwfeMPTwu3f
      QcF8LfeIl7u5M2PBfGKqEY2pEN1VfwFNA4N4PN8CgYAPfyVjjal5yvcKO7DaxmYC
      ywTaNwR9jsxAdezHGiDu/a9jaxerXMNtLt/m3OATafu9/T6JjkCGXclOPmYuhAEl
      ZBipZz/+1R1DqbRA5nqdrDDBp24bazWa3o/GztLF+U5TMhLuRlTmBvXAnak5YIHt
      fBPOndtQxZZ3HGGjofFKMQKBgA44KbsIluyOJPxWPScL7uGLN873QCRXJZHqObM9
      tABGRAOThr5Jm3KD4SF4jb0RDZ4p3n2t2QMR1FQ/I+XSQs6YfRTET5NhWXivzREt
      5VmYuvup3AJnRtmL65JgZ+ZBkl0Gvqk3X1bh13KmbffN62CmqXZXSVRHwVM84a4l
      7CXbAoGBALyCEhESCY4p6zcUej9M/7eGbqFIo/HXpfe0m/A93J6LwwRWSuDNqk2O
      r5qBJiAoVtuF9IlzLXKnkHo4oKS3EU0Fpe3zFkn1kluzPSWgfoEN7+QdXv3ppnYO
      1QEHVOsm4YyocfmEAdeBPW125nh12k7nZ79YUYCVqhF3jVFn4aH/
      -----END RSA PRIVATE KEY-----
    peer_ca_cert: |+
      -----BEGIN CERTIFICATE-----
      MIIE/zCCAumgAwIBAgIBATALBgkqhkiG9w0BAQswETEPMA0GA1UEAxMGcGVlckNB
      MB4XDTE1MDcxNjEzMjQxOFoXDTI1MDcxNjEzMjQyM1owETEPMA0GA1UEAxMGcGVl
      ckNBMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAuDFaTLJ//NLZUR8S
      gnKRh0vdjOfSwLakRfmWp/midwDFILuGvHgRd3ItsmNthy2ECQ3mr+zETAQ/Q3vp
      ba3P1hNMtCC1aHHnnF2KXqDCH9bYh7mqEhCUy3QXhJVWET2RgmWtvfXwPxr+hvxQ
      tjXhb9YloKkm99HNwREqczSUTZMmxirLbKnm7ztHtrUqMpaWiyKablgoukJCpufQ
      fOlKdxdX7fpQ5C2n+rYQWM2Xxu+KXeWv6E2MoZIYv+Gch2ZRWXC6fhQn7u8qSszZ
      reVMGbqsaQG+powLMOlA9ZW3KbIrf+jeNY5YFBWcPnGDNBZYgzud4x0i1BwfA7Mp
      T8fwjF1xEkmxB7Qf2gUZPEUDOgkDFszW2p9vEtqleMKJqSTMhxEMiwSB/CSVvGWI
      SclUHJN7pqcX2bKbGFWxMNfI/ez9lSDH7mqfRDPz/pLAvXLf5Xlsnzat50PKpBWt
      Wns1Z5KDeVMMn0MYu7gZ0GdA+/OotsP2r3BnmyPeiTQ0IlGz9Z7ikn/rZ+QfK8jf
      WGkQZlaQuNBUvC5UEn+I9n/qrTw38jUUY+IDDWOLp9VzpLNWIkSMKqJnN1igCZ/D
      QoW2rbqGwrv7UJywW1clglrS9nmOsGU9LtsU+KJeGRKK9lazkpujiKOLz306rIUU
      NBtbB1DDyvLTaj7Ln8VMD6v2BPkCAwEAAaNmMGQwDgYDVR0PAQH/BAQDAgAGMBIG
      A1UdEwEB/wQIMAYBAf8CAQAwHQYDVR0OBBYEFNixBensHx4NqEIf5jnCXZSXxnuH
      MB8GA1UdIwQYMBaAFNixBensHx4NqEIf5jnCXZSXxnuHMAsGCSqGSIb3DQEBCwOC
      AgEAhaHd/x1rAwkgIVEc+Y69vsrrpb2NOY6MB2ogLJnu8KaAcmvYsfku06Sc5GLn
      tXpkoftknrbjVV+g+XUhCz18NUY7YAFbYmembkC8ZVP32nQ1rsUf9jx8yiNYkeLq
      ZOYlnKbSram4/6Efg0ttxEgbIbwYPviApEH6DK26++vvxejgV+GdcMR9XXwEi/kN
      j1+ZfkzVnlO5j5uPLZi8vgsalJvWcPygolTxL73pfNXHj9QilxpUdJiVOvxke4MA
      VJOg8o02DN5QqRyT6oM1ivwbe7AYfZYRIjsJdSOXYvcBHk6iHZdPZeJcFnNjUOaE
      jvG/d9ezdUHa3C4qtHvmqcl2AjN/o50VyCY9/Mkgn8/tDOvVt3l3uSh0O4SQaZA1
      +KN7n0Jl0yiyv+3uGVWNOEX87SREcP0GbrsCdOGm3HmDTWw0UFidNJdzXkj2Iayv
      /hOq0PTBwTFm8shSXiPsjh6WMBXkkmu5FB51ZQ4Ch0MZDtuvlw9sGX9/zFNwL3W8
      Kqu6zV6ZSlv9RW9ChbHtDvs+DdqetU9WLYjglPcHfpV/BH1HRozfR1bStYm9Ljwy
      P8ZEmoycBR/79PtVdkSiFB4PiSkLHr6ICDSQGO+9+mLNQubFS+czQon90bZ9GVfg
      fvue6FeCS62q1lOmwKsNHi26szI5qY8b6Xj3cNjhDS5pIfg=
      -----END CERTIFICATE-----
    peer_cert: |+
      -----BEGIN CERTIFICATE-----
      MIIEbzCCAlmgAwIBAgIRAMR+bZyYqRB1XDKh7ZLkk2IwCwYJKoZIhvcNAQELMBEx
      DzANBgNVBAMTBnBlZXJDQTAeFw0xNTA3MTYxMzI0MjNaFw0xNzA3MTYxMzI0MjRa
      MCMxITAfBgNVBAMTGGV0Y2Quc2VydmljZS5jZi5pbnRlcm5hbDCCASIwDQYJKoZI
      hvcNAQEBBQADggEPADCCAQoCggEBALwfzvmk78lHrQuXF1PqgwyE+QNHALQf5peA
      O9mYDKDqqaTBNePuQZCZTDPCcqYPyQSPEX3RIhxR8OVKBubyOWFCe8y9CsbLwfq5
      /zXSeegYvW/OQoRa3BvlqezLSGIGDwmNciEUJEATl+wnumvDLnuhTsRsRHy5/RDA
      Su90VW3Uu0y1Tx5meFCtKiNxluLfo/CSj2Mo1FSn6BIpbajf5eailvmOImDJa2YZ
      stckVIro1+T6QkLuk2AAAmqXyGszjKLOaMEK45ys4cx/Cd5/FRen08C2JdyuGrDq
      SiVOKhFJ5TnGj/oDp+1R5SEAAIYttBrE8w098TQJBDdAoJDAnp0CAwEAAaOBszCB
      sDAOBgNVHQ8BAf8EBAMCALgwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMC
      MB0GA1UdDgQWBBQ5T85wynG27uRIwBb8PmfE5LixyjAfBgNVHSMEGDAWgBTYsQXp
      7B8eDahCH+Y5wl2Ul8Z7hzA/BgNVHREEODA2ghoqLmV0Y2Quc2VydmljZS5jZi5p
      bnRlcm5hbIIYZXRjZC5zZXJ2aWNlLmNmLmludGVybmFsMAsGCSqGSIb3DQEBCwOC
      AgEAdovxe3mCRqNSo/ghF7DKMnVUinNjelskq/FEBlU7DzrKd7b/WxJMz05CBcZh
      KMHrcBETvkzq2JBywxqRPAWZTv+2h5f8qSjjJUiHsQgXUD+vgxVGFvl+hwT5CJ6K
      aaj8v7ZqMe7CUg7lq857kkjVHHFieMsV6K/uDzFy0+TROHP1AUaUxKSsdohaySm+
      TCnpuW9rupuraqDOMiFdCL5rQ/zdNpYYa+qd3lTvXQJVUAnwLNmeFiv+eCxMHTdn
      hfPlvzhEegxCFZHLKNcdSxWVbix8ZLP4JcSges+DAQlXP96Zy/fYhZKp89SrI+Iq
      mZM/vx6I4XHq3zipZ6sFaRqSGaN5QuEU8gFCI18ODd7chefp7QOVDr3uNyrr6Qtg
      rwGHcbN9Xj2trRvk/CwZ2t9rVCYe1frcCvrUuJ+Ie5CQod5GHmI92P6u7/jyikwF
      TEXTjoF/rqsdim6SiyOullBkxGl3dCRhcJZzd2Twi2/xS01ysVfY5dLctg0tM/83
      RREZ7s40EieFNLulOsHuLu5+qWY7gh6JB1hRrGw1ni02qeuJ8SELGqwBHF0FtaFw
      F7V2pM0yBt/kLsqdzpjxUSrAQqV+IYAkXlmEJ5/k+rCkwYxFR/D/zmETsmPWzi++
      RHqTUB25ve+3l9sIRdrVNgJNJ/UWzaQ/Zstf1cIL3N1RehE=
      -----END CERTIFICATE-----
    peer_key: |+
      -----BEGIN RSA PRIVATE KEY-----
      MIIEowIBAAKCAQEAvB/O+aTvyUetC5cXU+qDDIT5A0cAtB/ml4A72ZgMoOqppME1
      4+5BkJlMM8Jypg/JBI8RfdEiHFHw5UoG5vI5YUJ7zL0KxsvB+rn/NdJ56Bi9b85C
      hFrcG+Wp7MtIYgYPCY1yIRQkQBOX7Ce6a8Mue6FOxGxEfLn9EMBK73RVbdS7TLVP
      HmZ4UK0qI3GW4t+j8JKPYyjUVKfoEiltqN/l5qKW+Y4iYMlrZhmy1yRUiujX5PpC
      Qu6TYAACapfIazOMos5owQrjnKzhzH8J3n8VF6fTwLYl3K4asOpKJU4qEUnlOcaP
      +gOn7VHlIQAAhi20GsTzDT3xNAkEN0CgkMCenQIDAQABAoIBADnfSyvPSpjP/PMA
      0wNUtGXojjYs5JGE8soOf9rrhI8IQZHWgj6RMAhMsH2Hxv9BAeTuIkJjUKwHpSTU
      RhVL1M0Px8fvK96GFjGMgG9NRYVZ/wTjHeFbljTazRB0ZNsK5BtbMQ3uBUzU+jqC
      6j12eNk9gV65s8Pu72P009igIBu9+1QGuV2r3qThd39oKrA10zuwlbw+P3NOuMqZ
      AlMzPzct20FuPYpwehP5bbXlcFdXxmjqCYI6oOYddeWwe/MiiCCbnsWj+Uvj9j4Z
      IIKbHUPqK0F2SPWEtDaLaWqGKXRxCRFpB8HSQ1Exi1WUvGImF4L6JFJyS8UTogSg
      0yTTf90CgYEAzBn175Iw9hpT9VgpvZee07YHtMzeerd9+9jmxwRvwDYQcxVksw7n
      VuTuq9Wwsec/rK3MAt1/hFj38paOr2FCPJ9o3plvaF4uPSjV6zQXG7J9iczNzdn2
      Cbz3739p9F6GzDjHPfquMI0ibxchj4oSeAfGzlFNnztXGRBlBJAdSKcCgYEA6/XP
      axJ/bl5r134LV7iTFuKjUSyBYtnRokFJsv6fh7LRRJ9b5W0OYEd5wfN08+7rSg/6
      F6yXKBvdLmcSLmn+yBTsO6DZWIe08ylgVBAvA3oSqzjzhnLrv+ZaJXHzPoP9bMC4
      TKqM7bAYJCliSGq18uIfle1qBpR6nlbvA7WwAxsCgYAmxROrg2ibhxrFsw6Svhdk
      feJu3K+yPeLHkUcdLOGRcHOleL3dKYqWPfx8VaYv1Q6KXaUwMiUD3eaThTfrZp0v
      aNSB3EGGYMWFxpkECawODdS89VNus+WBqgyqyNg2nDIc3vgx9Mlb3aNZ2Nn+Kysg
      89E25cjJ43rC/xNBT6LQZwKBgFV6NYpnKAyWXeCxg3Bip74pmdolEjX6DCwIFKen
      /6iLya1fQU4KRKPyIJR3Gk3npgqtYP7EgfmApo5Rvk9cDHT0x2MOcM3WU2GnAoNR
      XYaX6T1noyh4ZxicXNmlvuVNsTd9VQZI3kaYfRZUe4saRRFYgvKwD7GUhhroCSvB
      3KIzAoGBALcjCM1upXatDHncm6RXSPAabpJVIgK7H4CT0CoMFD/k9CoHXfpVQ0fd
      FtRlQt2oJnU/G0/sek1RCFdCWjGVklIPNwOyjXc3ZU4YPF+/WR07x1VLL14KEPN7
      QByXVUoXMbBBtmUyhVTizXhpNBP2dELMk+NVdnVWHCiGTtBAUTpk
      -----END RSA PRIVATE KEY-----
    server_cert: |+
      -----BEGIN CERTIFICATE-----
      MIIEcDCCAlqgAwIBAgIRANOoYOncPrjiL/mlurlyOFYwCwYJKoZIhvcNAQELMBIx
      EDAOBgNVBAMTB2RpZWdvQ0EwHhcNMTUwNzE2MTMyNDE2WhcNMTcwNzE2MTMyNDE3
      WjAjMSEwHwYDVQQDExhldGNkLnNlcnZpY2UuY2YuaW50ZXJuYWwwggEiMA0GCSqG
      SIb3DQEBAQUAA4IBDwAwggEKAoIBAQCiFGNIsTQxfjdZE5I544XpC0Yl+L1NIpN7
      uGBncsJYzyAQdG92UPr63sjkcwEnwXc1Ax4DnCRShLcWE7g8FvOsRhruSInPOb+3
      It3aT1YOK1Wct4PvcB/5HdpKaDiBQGsHDbLUnbO1EAujRkEpLarqfmDCNjKT1n+I
      VHYnR3AQkx7oE7Wt5Wf4cdCBgzo2o095j11r1zpWqae6xsMvheZ1S8Y3T0Iv6xJq
      V73amGqpCWQQaIOB4fM+sJilApptxZQ7qJivmv2zGJYpPer9TgfavVMXxGnnDOtl
      gqCZN11EIlZ2r1Li81YrBVtMSeqUYVy/sLHSaTkEmDUAZvC5AXZ9AgMBAAGjgbMw
      gbAwDgYDVR0PAQH/BAQDAgC4MB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcD
      AjAdBgNVHQ4EFgQUeFGbjkYXqkNkAEK3bKJeE8FpGFQwHwYDVR0jBBgwFoAUAU5p
      u7rUL87RDYhHRL+YYfgc/4YwPwYDVR0RBDgwNoIaKi5ldGNkLnNlcnZpY2UuY2Yu
      aW50ZXJuYWyCGGV0Y2Quc2VydmljZS5jZi5pbnRlcm5hbDALBgkqhkiG9w0BAQsD
      ggIBACJV1l26KSyctZZDaiZfvNB+ujlS2pIh9DL58NRHq4N9ETqqujRXMgI5oL1E
      iNJcc4b2LGR2c67KEr2gvLZiL/fXBCfotwqdUrR2KDFb9xV6f9H5Vq6rKH4+t528
      rTqKmEyGE+bXcqzh1pgCVUo2CxH/xJfarKMpCDNTFddLj3EUIuN6ossQI1sSCOwd
      jrdlednemTnrmHQUP5x+c1SXtiryc/bMZgCluKlVqwNrZcZFcw8G7cbdPryV+3a7
      G2xd6Kmf/TvVdYLRY0xbvxXO6EeATaAMgcqyv2Buc/1muhz9VGcpovh71vTLbfdk
      ZNUZV7+lIRWC0dTYyB/gyi4u6TmsVgk3TYjL5VR8eF7iiu/nLpdIRiVPG6LdKxxj
      sidESxOU5TzUFcEAO4cGv9yLqxDLqls94OMtc37HtyvPT9YFg1dJKw5bNcfoIzvf
      chyNtHL1/RgCQ7R7mWwjKikl2kGyheVcS6RqcRXV5S93xeGwQnCnF3UA/QvyGmXo
      o7AdbUCJQKfy9wciUJPQoQis7t1Ccojt7aojio3pK0yTp5jJmIi2hQ8G1hAloQ37
      7b21WF7S98HADj0Re8RhOXGj1WG0ginHrqd7P9whhWDMVfA9I2WvF0Yq73pCv3og
      v0NvtCOIdnDiVgl4surE9gs7LTeeHyU8RDt4L1MY8reqlVtO
      -----END CERTIFICATE-----
    server_key: |+
      -----BEGIN RSA PRIVATE KEY-----
      MIIEpQIBAAKCAQEAohRjSLE0MX43WROSOeOF6QtGJfi9TSKTe7hgZ3LCWM8gEHRv
      dlD6+t7I5HMBJ8F3NQMeA5wkUoS3FhO4PBbzrEYa7kiJzzm/tyLd2k9WDitVnLeD
      73Af+R3aSmg4gUBrBw2y1J2ztRALo0ZBKS2q6n5gwjYyk9Z/iFR2J0dwEJMe6BO1
      reVn+HHQgYM6NqNPeY9da9c6VqmnusbDL4XmdUvGN09CL+sSale92phqqQlkEGiD
      geHzPrCYpQKabcWUO6iYr5r9sxiWKT3q/U4H2r1TF8Rp5wzrZYKgmTddRCJWdq9S
      4vNWKwVbTEnqlGFcv7Cx0mk5BJg1AGbwuQF2fQIDAQABAoIBAQCfayhAnrN0nu23
      us1QDR9wmjs0LBWeIg0oWrDP74uDKK8kIDJmEL7cNHcqZIfVX7BtvxQtfs4nMAyZ
      NWo4CGdCom3oxAZwgh+09SF7kh9Vrn/1tneZ8hIwyJEmMJ6rWv4qoOmtwTO6Ov8H
      aJm89AMxxH5NaFuVGBy2rkTM27I5SbsTDj8YMz5unBXlbY+BaZG2hzZCdZIgLwoL
      TycZYjfxaK8fcOxJBrIC5QR75YKgSFdPFLUyUjf3t7G+f4y4RSV8VQ4GqW6cFeGD
      kENo80yYqnRxlfsBxGO3uV/iCOP5QqvyYyxvWsXVhlG1ml8ITsbn70H5gO7XkOYX
      wJ+HWi/NAoGBAMjOWQ/y0XPPNFJ+1FH7pDMchX1VYFiY3788u5WIJ/nrJS6WGtFS
      L94GHn5Ruhak0GG7VhFqBBgmh+lhZwIilZ8msgqtlJkCUEJXFq9F04fQx4QLAj+B
      eZo0a5vTl1PzLq807w/AkzxVDT+PMhyA7IRImJMhIxk7vYP+CqdTrTR/AoGBAM6h
      EaXqaFz7jiIiXHkjhnmpufrM92roBRS+hLmlsZSA1t1Np4yA1iN0g6B9Fdqo0z5n
      6YsOqs90WCEIJqDoitwLWKF+IRN3Lvc43cVB079CeREfKww6I4U/kr64DD+3PIYM
      PW/zCRpSt814gbcupDERchyuD7oC/RG1xvslxacDAoGBAKJx07jELU7ri59E/MwJ
      r06tvwuiOpvRqAfTwMh56iUSZfTm93DodNK+zoJP6SOSVwUJANp7ki5bVU2mPyeK
      BNJIAnYC8BhLt9PDEhXeff38FrsqELqBKndl+ruHk38VVmnkf5SVrEZ9Y4dMdzR5
      01w8Qjmb8AHkwy55H/M3DQJPAoGAC688Cj/ZKvjmrrN2uzrxDcw1QiN5EkiQkP29
      D6p5AkbO37DWerGGanbaQqcQJ09Issy5fi2UJysTGLsXRB4iTBMwLeGuCSXCOCS1
      FcSFLtmZcwhqLMTU4WIY8EQEHU5FU+c5Si1aJGztC+d2nl861bOA2nJVXVVx7iBz
      YhxesvUCgYEAl2Wg9mfN1rrnvotQ37aNkkDy7Tr6eLZTYWYlSV8Eit6db28gxt25
      cykNYAMddd5xUDyB2Bg/gCvmzOEcZYIhdqbFVFpb1HxoZ7L7eC6u7+kyvAXbF3at
      CscMkuvB4+005RWJIN9GJbQf1+OsQbKotX0OerT2aCYaz3g/Y3mZdWU=
      -----END RSA PRIVATE KEY-----
`
