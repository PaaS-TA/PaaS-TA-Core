#!/bin/bash
set -x -v

while getopts ":z:c:s:i" opt; do
    case $opt in
	z)
	    identity_zone=$OPTARG
	    ;;
	c)
	    client_id=$OPTARG
	    ;;
	s)
	    client_secret=$OPTARG
	    ;;
        i)
            skip_ssl="true"
            ;;
	\?)
	    echo "Invalid option: -$OPTARG" >&2
	    exit 1
	    ;;
	:)
	    echo "Option -$OPTARG requires an argument." >&2
	    exit 1
	    ;;
    esac
done

if [[ -z "$identity_zone" ]]; then
    echo "You must specify an identity zone with option -z."
    exit 1
fi

if [[ -z "$client_id" ]]; then
    echo "You must specify a client id with option -c."
    exit 1
fi

if [[ -z "$client_secret" ]]; then
    echo "You must specify the client secret with option -s."
    exit 1
fi

function create_zone_admin_client {
    loc_identity_zone=$1
    loc_client_id=$2
    loc_client_secret=$3

    # Check that the current token has sufficient privileges to create the client id.
    zone_admin_authority="zones.$loc_identity_zone.admin"
    token_scope=`uaac token decode | grep "scope:"`

    payload='{ "client_id" : "'"$loc_client_id"'", "client_secret" : "'"$loc_client_secret"'", "authorized_grant_types" : ["client_credentials"], "scope" : ["uaa.none"], "authorities" : ["clients.admin", "clients.read", "clients.write", "clients.secret", "zones.'"$loc_identity_zone"'.admin", "scim.read", "scim.write", "idps.read", "idps.write", "uaa.resource"], "resource_ids" : ["none"], "allowedproviders" : ["uaa"]}'

    if [[ -z $skip_ssl ]]; then
        uaac curl -XPOST -H "Accept: application/json" -H "Content-Type: application/json" -H "X-Identity-Zone-Id: $loc_identity_zone" -d "$payload" /oauth/clients
    else
        uaac curl -XPOST -H "Accept: application/json" -H "Content-Type: application/json" -H "X-Identity-Zone-Id: $loc_identity_zone" -d "$payload" /oauth/clients --insecure
    fi
}

create_zone_admin_client $identity_zone $client_id $client_secret

