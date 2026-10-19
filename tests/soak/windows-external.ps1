# [INPUT]: Isolated soak/server executables and a new output directory.
# [OUTPUT]: Separate WebView host and HTTP server handle measurements.
# [POS]: Diagnostic ownership split; only the created server process is stopped.
# [PROTOCOL]: Update tests/soak/AGENTS.md; keep evidence under the isolated test root.
param(
    [Parameter(Mandatory=$true)][string]$Executable,
    [Parameter(Mandatory=$true)][string]$Server,
    [Parameter(Mandatory=$true)][string]$Output,
    [ValidateRange(1,86400)][int]$Duration = 300,
    [ValidateRange(1,3600)][int]$Recovery = 90
)
$ErrorActionPreference = 'Stop'
$Server = (Resolve-Path -LiteralPath $Server).Path
$Output = [IO.Path]::GetFullPath($Output)
if (Test-Path -LiteralPath $Output) { throw 'Evidence directory must be new' }
New-Item -ItemType Directory -Path $Output | Out-Null
$serverOutput = Join-Path $Output 'server'
$process = Start-Process -FilePath $Server -PassThru -ArgumentList @("-output=`"$serverOutput`"", "-duration=$($Duration+$Recovery+180)s") `
    -RedirectStandardOutput (Join-Path $Output 'server.stdout.log') -RedirectStandardError (Join-Path $Output 'server.stderr.log')
try {
    $originsFile = Join-Path $serverOutput 'origins.json'
    $deadline = [DateTime]::UtcNow.AddSeconds(20)
    while (-not (Test-Path -LiteralPath $originsFile)) {
        if ($process.HasExited -or [DateTime]::UtcNow -gt $deadline) { throw 'HTTP server startup failed' }
        Start-Sleep -Milliseconds 100
    }
    $origin = Get-Content -LiteralPath $originsFile -Raw | ConvertFrom-Json
    & (Join-Path $PSScriptRoot 'windows.ps1') -Executable $Executable -Output (Join-Path $Output 'host') `
        -Duration $Duration -Recovery $Recovery -Origins $origin.origins
} finally {
    if (-not $process.HasExited) {
        New-Item -ItemType File -Path (Join-Path $serverOutput 'stop') -Force | Out-Null
        if (-not $process.WaitForExit(10000)) { $process.Kill(); $process.WaitForExit() }
    }
    $process.Dispose()
}
