$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(2) }

$Address = "http://127.0.0.1:6769/health_check"
Invoke-WebRequest -UseBasicParsing "${Address}"

# Go struggles to capture the exit codes of
# Windows commands/scripts that exit quickly
Start-Sleep -Seconds 3

Exit 0
