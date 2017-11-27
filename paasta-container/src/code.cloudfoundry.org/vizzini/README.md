# Vizzini

**Note**: This repository should be imported as `code.cloudfoundry.org/vizzini`.

[Inconceivable tests!](http://www.imdb.com/character/ch0003791/)

Vizzini is a suite of tests that runs against the Diego BBS API.

## What's In Here

- Under the top-level directory are tests that exercise the core of Diego and
  the HTTP routing tier through a variety of use-cases. Although they are
  primarily used to accept stories related to the details of Task and LRP
  behavior, they are a valuable integration suite for Diego as a whole. Also,
  they are fast and can safely be run in parallel.

## How to use

The following assumes [diego-release](https://github.com/cloudfoundry/diego-release) is cloned at `/path/to/diego-release`.

### As a Bosh Errand

If you are using old manifest generation, simply run the following commands to generate the vizzini manifest:

``` shell
/path/to/diego-release/scripts/generate-vizzini-manifest \
    -c /path/to/cf/manifest.yml \
    -p /path/to/vizzini/property-overrides.yml \
    -i /path/to/vizzini/iaas-settings.yml > \
    /path/to/vizzini/manifest.yml

bosh deployment /path/to/vizzini/manifest.yml
bosh deploy
bosh run errand diego-vizzini
```

If you are using [cf-deployment](https://github.com/cloudfoundry/cf-deployment/) you can simply deploy again using [this operations file](https://github.com/cloudfoundry/diego-release/blob/develop/operations/add-vizzini-errand.yml), e.g.:


``` shell
bosh -d cf deployment -o /path/to/diego-release/operations/add-vizzini-errand.yml ....
bosh -d cf run-errand vizzini
```

### Standalone

this is only supported when deploying to bosh-lite using the manifest generation method:

``` shell
/path/to/diego-release/scripts/run-vizzini-bosh-lite
```

#### Learn more about Diego and its components at [diego-design-notes](https://github.com/cloudfoundry/diego-design-notes)
