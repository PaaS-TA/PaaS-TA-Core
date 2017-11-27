#!/bin/bash
#set -x

while getopts ":z:d:n:x:i" opt; do
    case $opt in
	z)
	    zone_id=$OPTARG
	    ;;
	d)
	    zone_subdomain=$OPTARG
	    ;;
	n)
	    zone_name=$OPTARG
	    ;;
	x)
	    zone_description=$OPTARG
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

if [[ -z "$zone_id" ]]; then
    echo "You must specify the zone id with option -z."
    exit 1
fi

if [[ -z "$zone_subdomain" ]]; then
    echo "You must specify the zone subdomain with option -d."
    exit 1
fi

if [[ -z "$zone_name" ]]; then
    echo "You must specify the zone name with option -n."
    exit 1
fi

if [[ -z "$zone_description" ]]; then
    echo "You must specify the zone description with option -x."
    exit 1
fi

payload='{"id":"'"$zone_id"'","subdomain":"'"$zone_subdomain"'","name":"'"$zone_name"'","description":"'"$zone_description"'"  }'

if [[ -z $skip_ssl ]]; then
    uaac curl -X POST -H "Accept:application/json" -H "Content-Type:application/json" /identity-zones -d "$payload"
else
    uaac curl -X POST -H "Accept:application/json" -H "Content-Type:application/json" /identity-zones -d "$payload" --insecure
fi
