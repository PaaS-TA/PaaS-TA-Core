#!/bin/bash
#Usage - . ./create_uaa_zone_client.sh <Number of Identity zones to be created>
host=http://localhost:8080/uaa
admin_client_secret=adminsecret
admin_client_id=admin

if [[ $# -lt 1 ]]
then
  echo " Please provide the number of Identity zones to be created.. Exiting script"
  exit
fi
uaac target $host
uaac token client get admin -s $admin_client_secret
zoneid=1
while [ $zoneid -lt `expr $1 + 1` ]
do
  ### Setup UAA Zone and admin client ####
  ./create-zone.sh -z perfzone$zoneid -d perfzone$zoneid -n "Performance test zone $zoneid" -x "Performance zone"
  uaac client update admin --authorities clients.read,zones.read,clients.secret,zones.write,clients.write,clients.admin,uaa.admin,scim.write,scim.read,zones.perfzone$zoneid.admin
  uaac token client get admin -s $admin_client_secret
  ./create-zone-admin-client.sh -z perfzone$zoneid -c zoneclient$zoneid -s clientsecret
  zoneid=`expr $zoneid + 1`
done
