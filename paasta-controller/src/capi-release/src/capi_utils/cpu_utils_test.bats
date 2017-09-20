#!/usr/bin/env bats

setup() {
  ## override stubs with original functions
  source ./cpu_utils.sh

  TMPDIR=$(mktemp -dt "cpu_test.XXXXXXX")
  PIDFILE="${TMPDIR}/test.pid"
  PATH_OOO=${PATH}
  export PATH=${TMPDIR}:${PATH}
}

teardown() {
  rm -rf ${TMPDIR}
  PATH=${PATH_OOO}
}

stubUname() {
  cat >"${TMPDIR}/uname" <<EOL
#!/usr/bin/env bash
echo "$1"
EOL
chmod 0755 "${TMPDIR}/uname"
}

stubSysctl() {
  cat >"${TMPDIR}/sysctl" <<EOL
#!/usr/bin/env bash
args=(\$*)
echo "\${args[0]} \${args[1]}" > "${TMPDIR}/sysctl_usage"

echo 27
EOL
chmod 0755 "${TMPDIR}/sysctl"
}

stubGrepProcfs() {
  cat >"${TMPDIR}/grep" <<EOL
#!/usr/bin/env bash
args=(\$*)
echo "\${args[0]} \${args[1]} \${args[2]}" > "${TMPDIR}/grep_usage"

echo 48
EOL
chmod 0755 "${TMPDIR}/grep"
}

###
### cpu_count
###

@test "cpu_count uses sysctl on Darwin" {
  stubUname "Darwin"
  stubSysctl
  run cpu_count
  [ "$status" -eq 0 ]
  [ "$output" == "27" ]
  [ "$(cat "${TMPDIR}/sysctl_usage")" == "-n hw.ncpu" ]
}

@test "cpu_count uses grep with procfs on Linux" {
  stubUname "Linux"
  stubGrepProcfs
  run cpu_count
  [ "$status" -eq 0 ]
  [ "$output" == "48" ]
  [ "$(cat "${TMPDIR}/grep_usage")" == "-c ^processor /proc/cpuinfo" ]
}

