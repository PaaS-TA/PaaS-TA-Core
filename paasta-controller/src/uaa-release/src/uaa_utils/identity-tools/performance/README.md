README
# UAA Performance Test

## Prerequisites
1. Download Jmeter and add the bin directory to the path
1. `gradle`,`mysql`, `mysqlimport` on the cli

## Run data-load-scripts

Perform the following gradle tasks to CSV load performance data into the target mysql db.
1. `cd data-load-scripts`
1. Update the DB password in `gradle.properties`. ONLY change the password. Please note that the password is for uaa aws acceptance db.
1. If the environment requires SSH tunneling, run `gradle startSSH -Pfile=path_to_pem_file`
1. Then run `gradle createAndLoad -Pcount=num_of_zones,num_of_clients_per_zone,num_of_users_per_zone` to create the CSV and import them to the target db
1. To delete performance data from the db use `gradle cleandb`
1. If SSH tunnel was established earlier, `gradle stopSSH` can be used to close it

## Run `hey` performance tool
1. Run the performance scripts from dedicated VM with name `hey go` and `hey go 2` created in acceptance AWS for this purpose
1. `ssh ubuntu@<ip address of the vm> -i path_to_bosh_acceptance.pem`
1. The details for executing the `hey` scripts can be found in the [uaa-hey](https://github.com/cloudfoundry/uaa-hey) project
