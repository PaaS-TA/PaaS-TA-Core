#!/usr/bin/env bash

running_in_container() {
  grep -q -E '/instance|/docker/' /proc/self/cgroup
}

