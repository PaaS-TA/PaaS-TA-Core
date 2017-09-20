# a volman driver for local disk
This driver is intended for test purposes only.  Watch this space in case that changes...

#Usage of localdriver:
----
```
  -caFile string
        the certificate authority public key file to use with ssl authentication
  -certFile string
        the public key file to use with ssl authentication
  -clientCertFile string
        the public key file to use with client ssl authentication
  -clientKeyFile string
        the private key file to use with client ssl authentication
  -debugAddr string
        host:port for serving pprof debugging info
  -driversPath string
        Path to directory where drivers are installed
  -insecureSkipVerify
        whether SSL communication should skip verification of server IP addresses in the certificate
  -keyFile string
        the private key file to use with ssl authentication
  -listenAddr string
        host:port to serve volume management functions (default "0.0.0.0:9750")
  -logLevel string
        log level: debug, info, error or fatal (default "info")
  -mountDir string
        Path to directory where fake volumes are created (default "/tmp/volumes")
  -requireSSL
        whether the fake driver should require ssl-secured communication
  -transport string
        Transport protocol to transmit HTTP over (default "tcp")
```

# Specific Options
----

## Create
```
voldriver.CreateRequest{
    Name: "Volume",
    Opts: map[string]interface{}{
        "volume_id": "something_different_than_test",
        "passcode" : "someStringPasscode",              <- OPTIONAL
    },
})
```

## Mount
```
localDriver.Mount(logger, voldriver.MountRequest{
    Name: "Volume",
    Opts: map[string]interface{}{
        "passcode":"someStringPasscode"                 <- REQUIRED if used in Create
    },
})
```