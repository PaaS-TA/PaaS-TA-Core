#!/bin/bash

RELEASE=$(cd $(dirname $0)/.. && pwd)

ln -fs ${RELEASE}/git-hooks/pre-commit ${RELEASE}/../../../../.git/modules/src/github.com/cloudfoundry-incubator/volman/hooks/pre-commit
#ln -fs ${RELEASE}/git-hooks/pre-push ${RELEASE}/.git/hooks/pre-push
