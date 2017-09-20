#!/usr/bin/env bash

function cpu_count() {
  declare platform
  platform=$(uname)
  case "${platform}" in
    Darwin)
      sysctl -n hw.ncpu
      ;;
    Linux)
      grep -c ^processor /proc/cpuinfo
      ;;
  esac
}
