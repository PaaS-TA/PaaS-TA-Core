# package systemcerts

This repo is a copy of the Go1.6 `crypto/x509` package. There is an unfortunate bug [#13335](https://github.com/golang/go/issues/13335)
which causes use to not add custom certs to golang brojects via manifest configurations. 
The bug has been fixed for future versions of golang, but untill that fix makes it's way into
a stable release we will be using this repository as a replacment for golang's `crypto/x509` package.
