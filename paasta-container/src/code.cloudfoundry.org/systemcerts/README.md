# package systemcerts

**Note**: This repository should be imported as `code.cloudfoundry.org/systemcerts`.

This repo is a copy of the Go1.6 `crypto/x509` package. There is an unfortunate bug [#13335](https://github.com/golang/go/issues/13335)
which causes use to not add custom certs to golang brojects via manifest configurations. 
The bug has been fixed for future versions of golang, but until that fix makes it's way into
a stable release we will be using this repository as a replacement for golang's `crypto/x509` package.

The repo also contains code from an [additional commit](https://github.com/golang/go/commit/05471e9ee64a300bd2dcc4582ee1043c055893bb) to the
Golang 1.8 `crypto/x509` library. This code implements the SystemCertPool on Windows, but was reverted in a [later commit](https://github.com/golang/go/commit/2c8b70eacfc3fd2d86bd8e4e4764f11a2e9b3deb) due to
some edge cases that don't really apply to BOSH-deployed jobs (see [Issue #18609](https://github.com/golang/go/issues/18609) for details). These edge cases are expected to be fixed in a future version of Golang
(ideally Go 1.9).
