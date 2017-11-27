# gosub

`$GOPATH` submodule automation.


## usage

### show all packages required by a set of applications or test suites

The `list` command simply lists out a full set of dependent packages,
determined by the given app or test packages.

```sh
$ gosub list -app github.com/my/app -test github.com/my/tests
github.com/my/app
github.com/my/app/cmd/foo
github.com/my/lib
github.com/my/tests
github.com/onsi/ginkgo
github.com/onsi/gomega
# etc...
```

This is intended to be composed with other tooling, i.e. `sync` or automating
lists of dependant files.


### synchronize submodule config with a given set of packages

The `sync` command synchronizes submodules under a `$GOPATH` embedded in a repo.

For each package, it will find the root of its Git repo under the `$GOPATH`,
and add it as a submodule.

Non-Git packages are added individually, vendored into the parent repo. You may
want to add `.hg` and `.bzr` to your `.gitignore`.

Any extra submodules under the `$GOPATH` are then removed. This is to prune
no-longer-needed dependencies. Specify the '-i' flag for each submodule you 
would like to avoid pruning.

```sh
$ gosub sync github.com/my/app github.com/my/lib ...
```

This command works well with `gosub list`:

```sh
$ gosub list -a github.com/my/app | xargs gosub sync
```
