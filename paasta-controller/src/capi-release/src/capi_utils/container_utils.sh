#!/usr/bin/env bash

running_in_container() {
  # look for a non-root cgroup
  grep --quiet --invert-match ':/$' /proc/self/cgroup
}

