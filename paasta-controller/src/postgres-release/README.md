# postgres-release
---

This is a [BOSH](https://www.bosh.io) release for [PostgreSQL](https://www.postgresql.org/).

## Contents

* [Deploying](#deploying)
* [Customizing](#customizing)
* [Contributing](#contributing)
* [Known Limitation](#known-limitation)
* [Upgrading](#upgrading)

## Deploying

In order to deploy the postgres-release you must follow the standard steps for deploying software with BOSH.

1. Install and target a BOSH director.
   Please refer to [BOSH documentation](http://bosh.io/docs) for instructions on how to do that.
   Bosh-lite specific instructions can be found [here](https://github.com/cloudfoundry/bosh-lite).

1. Install the BOSH command line Interface (CLI) v2+.
   Please refer to [BOSH CLI documentation](https://bosh.io/docs/cli-v2.html#install).

1. Upload the desired stemcell directly to bosh. [bosh.io](http://bosh.io/stemcells) provides a resource to find and download stemcells.
   For bosh-lite:

   ```
   bosh upload-stemcell https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent
   ```

1. Upload the latest release from [bosh.io](http://bosh.io/releases/github.com/cloudfoundry/postgres-release?all=1):

   ```
   bosh upload-release https://bosh.io/d/github.com/cloudfoundry/postgres-release
   ```

   or create and upload a development release:

   ```
   cd ~/workspace/postgres-release
   bosh -n create-release --force && bosh -n upload-release
   ```

1. Generate the manifest. You can provide in input an [operation file](https://bosh.io/docs/cli-ops-files.html) to customize the manifest:

   ```
   ~/workspace/postgres-release/scripts/generate-deployment-manifest-v2 \
   -o OPERATION-FILE-PATH > OUTPUT_MANIFEST_PATH

   ```

   You can use the operation file to specify postgres properties ( see by way of [example](blob/master/templates/v2/operations/set_properties.yml)) or to override the configuration if your BOSH director [cloud-config](http://bosh.io/docs/cloud-config.html) is not compatible.

   You are also provided with options to enable ssl in the PostgreSQL server or to use static ips.


1. Deploy:

   ```
   bosh -d DEPLOYMENT_NAME deploy OUTPUT_MANIFEST_PATH
   ```

## Customizing

The table below shows the most significant properties you can use to customize your postgres installation.
The complete list of available properties can be found in the [spec](blob/master/jobs/postgres/spec).

Property | Description
-------- | -------------
databases.port | The database port
databases.databases | A list of databases and associated properties to create when Postgres starts
databases.databases[n].name | Database name
databases.databases[n].citext | If `true` the citext extension is created for the db
databases.databases[n].run\_on\_every_startup | A list of SQL commands run at each postgres start against the given database as `vcap`
databases.roles | A list of database roles and associated properties to create
databases.roles[n].name | Role name
databases.roles[n].password | Login password for the role. If not provided, SSL certificate authentication is assumed for the user.
databases.roles[n].common_name| The cn attribute of the certificate for the user. It only applies to SSL certificate authentication.
databases.roles[n].permissions| A list of attributes for the role. For the complete list of attributes, refer to [ALTER ROLE command options](https://www.postgresql.org/docs/9.4/static/sql-alterrole.html).
databases.tls.certificate | PEM-encoded certificate for secure TLS communication
databases.tls.private_key | PEM-encoded key for secure TLS communication
databases.tls.ca | PEM-encoded certification authority for secure TLS communication. Only needed to let users authenticate with SSL certificate.
databases.max_connections | Maximum number of database connections
databases.log_line\_prefix | The postgres `printf` style string that is output at the beginning of each log line. Default: `%m:`
databases.collect_statement\_statistics | Enable the `pg_stat_statements` extension and collect statement execution statistics. Default: `false`
databases.additional_config | A map of additional key/value pairs to include as extra configuration properties
databases.monit_timeout | Monit timout in seconds for the postgres job start. By default the global monit timeout applies. You may need to specify a higher value if you have a large database and the postgres-release deployment includes a PostgreSQL upgrade.

*Note*
- Removing a database from `databases.databases` list and deploying again does not trigger a physical deletion of the database in PostgreSQL.
- Removing a role from `databases.roles` list and deploying again does not trigger a physical deletion of the role in PostgreSQL.

### Enabling SSL on the PostgreSQL server
PostgreSQL has native support for using SSL connections to encrypt client/server communications for increased security.
You can enable it by setting the `databases.tls.certificate` and the `databases.tls.private_key` properties.

A script is provided that creates a CA, generates a keypair, and signs it with the CA:

```
./scripts/generate-postgres-certs -n HOSTNAME_OR_IP_ADDRESS
```
 The common name for the server certificate  must be set to the DNS hostname if any or to the ip address of the PostgreSQL server. This because in ssl mode 'verify-full', the hostname is matched against the common-name. Refer to [PostgreSQL documentation](https://www.postgresql.org/docs/9.6/static/libpq-ssl.html) for more details.

 You can also use [BOSH variables](https://bosh.io/docs/cli-int.html) to generate the certificates. See by way of [example](blob/master/templates/v2/operations/use_ssl.yml) the operation file used by the manifest generation script. 

```
~/workspace/postgres-release/scripts/generate-deployment-manifest-v2 \
   -s -h HOSTNAME_OR_IP_ADDRESS \
   -o OPERATION-FILE-PATH > OUTPUT_MANIFEST_PATH

```
### Enabling SSL certificate authentication

In order to perform authentication using SSL client certificates, you must not specify a user password and you must configure the following properties:

- `databases.tls.certificate`
- `databases.tls.private_key`
- `databases.tls.ca`

The cn (Common Name) attribute of the certificate will be compared to the requested database user name, and if they match the login will be allowed. 

Optionally you can map the common_name to a different database user by specifying property `databases.roles[n].common_name`.

A script is provided that creates a client certificates:

```
./scripts/generate-postgres-client-certs --ca-cert <PATH-TO-CA-CERT> --ca-key <PATH-TO-CA-KEY> --client-name <USER_NAME>
```


## Contributing

### Contributor License Agreement

Contributors must sign the Contributor License Agreement before their contributions can be merged.
Follow the directions [here](https://www.cloudfoundry.org/community/contribute/) to complete that process.

### Developer Workflow

1. [Fork](https://help.github.com/articles/fork-a-repo) the repository and make a local [clone](https://help.github.com/articles/fork-a-repo#step-2-create-a-local-clone-of-your-fork)
1. Create a feature branch from the development branch

   ```bash
   cd postgres-release
   git checkout develop
   git checkout -b feature-branch
   ```
1. Make changes on your branch
1. Test your changes by running [acceptance tests](blob/master/docs/acceptance-tests.md)
1. Push to your fork (`git push origin feature-branch`) and [submit a pull request](https://help.github.com/articles/creating-a-pull-request) selecting `develop` as the target branch.
   PRs submitted against other branches will need to be resubmitted with the correct branch targeted.

## Known Limitations

The postgres-release does not directly support high availability.
Even if you deploy more instances, no replication is configured.

## Upgrading

Refer to [versions.yml](blob/master/versions.yml) in order to assess if you are upgrading to a new PostgreSQL version.

### Considerations before deploying

1. A copy of the database is made for the upgrade, you may need to adjust the persistent disk capacity of the postgres job.
1. The upgrade happens as part of the monit start and its duration may vary basing on your env. The postgres monit start timeout can be adjusted using property `databases.monit_timeout`. You may need to specify a higher value if you have a large database
    - In case of a PostgreSQL minor upgrade a simple copy of the old data directory is made.
    - In case of a PostgreSQL major upgrade the `pg_upgrade` utility is used.
1. Postgres will be unavailable during this upgrade.

### Considerations after a successfull deployment

Post upgrade, both old and new databases are kept. The old database moved to `/var/vcap/store/postgres/postgres-previous`. The postgres-previous directory will be kept until the next postgres upgrade is performed in the future. You are free to remove this if you have verified the new database works and you want to reclaim the space.

### Recovering a failure during deployment

In case the timeout was not sufficient, the deployment would fail; anyway monit would not stop the actual upgrade process. In this case you can just wait for the upgrade to complete and only when postgres is up and running rerun the bosh deploy.

If the upgrade fails:

- The old data directory is still available at `/var/vcap/store/postgres/postgres-x.x.x` where x.x.x is the old PostgreSQL version
- The new data directory is at `/var/vcap/store/postgres/postgres-y.y.y` where y.y.y is the new PostgreSQL version
- If the upgrade is a PostgreSQL major upgrade:
  - A marker file is kept at `/var/vcap/store/postgres/POSTGRES_UPGRADE_LOCK` to prevent the upgrade from happening again.
  - `pg_upgrade` logs that may have details of why the migration failed can be found in `/var/vcap/sys/log/postgres/postgres_ctl.log`

If you want to attempt the upgrade again or to rollback to the previous release, you should remove the new data directory and, if present, the marker file.

