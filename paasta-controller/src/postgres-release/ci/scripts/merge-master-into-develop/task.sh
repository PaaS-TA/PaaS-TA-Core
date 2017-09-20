#!/bin/bash -exu

MERGED_REPO="${PWD}/${MERGED_REPO:?"MERGED_REPO required"}"
MASTER_BRANCH="${MASTER_BRANCH:-master}"

# Cannot set -u before sourcing .bashrc because of all
# the unbound variables in things beyond our control.
set +u
source ~/.bashrc
set -u

pushd release-repo > /dev/null
  git config user.name "CF MEGA BOT"
  git config user.email "cf-mega@pivotal.io"

  git remote add -f master-repo ../release-repo-master
  git merge --no-edit "master-repo/${MASTER_BRANCH}"

  git status
  git show --color | cat
popd > /dev/null

shopt -s dotglob
cp -R release-repo/* $MERGED_REPO
