$Address = "http://127.0.0.1:6769/health_check"
$ExitCode = 0
try {
    Invoke-WebRequest "${Address}"
} catch {
    Write-Error "Error talking to: ${Address}"
    Write-Error $_.Exception.Message
    $ExitCode = 2
}

# Go struggles to capture the exit codes of
# Windows commands/scripts that exit quickly
Start-Sleep -Seconds 3

Exit $ExitCode
