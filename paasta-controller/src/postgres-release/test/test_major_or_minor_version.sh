#!/bin/bash -e
pgversion_upgrade_from=$1
pgversion_current=$2

# From postgres-x.y.z, it's major if x and y are not the same
# in $pgversion_current and $pgversion_upgrade_from

function is_major() {
  [ "${pgversion_current%.*}" != "${pgversion_upgrade_from%.*}" ]
}
if is_major; then
 echo is major
else
 echo is minor
fi
