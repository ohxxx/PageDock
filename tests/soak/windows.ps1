# [INPUT]: An isolated test EXE built by run.py --windows-build and an unused evidence directory.
# [OUTPUT]: Go behavior logs plus private bytes, working set, handles, threads, GUI resources and WebView2 processes.
# [POS]: Windows desktop endurance runner; only this run's profile and process tree may be cleaned up.
# [PROTOCOL]: Update tests/soak/AGENTS.md and tests/AGENTS.md when isolation or measurements change.
param(
    [Parameter(Mandatory=$true)][string]$Executable,
    [Parameter(Mandatory=$true)][string]$Output,
    [ValidateRange(1,86400)][int]$Duration = 1800,
    [ValidateRange(1,3600)][int]$Recovery = 300,
    [ValidateRange(0.1,60)][double]$Pace = 3,
    [int]$Batch = 128,
    [ValidateRange(1,100)][int]$Visits = 3,
    [string]$TestPattern = '^TestDesktopResourceSoak$',
    [string]$Origins = '',
    [switch]$ClosePageFirst,
    [switch]$BareWebView,
    [switch]$BareEval,
    [switch]$BareTaskbar,
    [switch]$BareBackground,
    [switch]$BareRender,
    [switch]$BareLoading,
    [switch]$BareBridge,
    [ValidateRange(0,60)][double]$BareDwell = 0
)
$ErrorActionPreference = 'Stop'
if ((($BareEval -or $BareTaskbar -or $BareBackground -or $BareRender -or $BareLoading -or $BareBridge -or $BareDwell -gt 0) -and -not $BareWebView) -or ($BareWebView -and $ClosePageFirst)) { throw 'Bare controls require BareWebView and exclude ClosePageFirst' }
if ($Batch -lt 0 -or ($Batch -gt 0 -and $Batch -lt 4)) { throw 'Batch must be zero or at least four' }
$Executable = (Resolve-Path -LiteralPath $Executable).Path
$Output = [IO.Path]::GetFullPath($Output)
if (Test-Path -LiteralPath $Output) { throw 'Evidence directory must be new' }
New-Item -ItemType Directory -Path $Output | Out-Null
$profileRoot = Join-Path $Output 'profile'
New-Item -ItemType Directory -Path $profileRoot | Out-Null
$saved = @{}
foreach ($key in @('LOCALAPPDATA','WEBVIEW2_USER_DATA_FOLDER','WEBVIEW2_BROWSER_EXECUTABLE_FOLDER','WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS')) {
    $saved[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
}
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class PageDockGuiResources {
    [DllImport("user32.dll")] public static extern uint GetGuiResources(IntPtr handle, uint flag);
}
'@
$process = $null
$log = $null
$watch = [Diagnostics.Stopwatch]::StartNew()
function Owned-WebViews {
    @(Get-CimInstance Win32_Process -Filter "Name = 'msedgewebview2.exe'" | Where-Object {
        $_.CommandLine -and $_.CommandLine.IndexOf($profileRoot, [StringComparison]::OrdinalIgnoreCase) -ge 0
    })
}
function Sample-Process($item) {
    $item.Refresh()
    [ordered]@{ pid=$item.Id; private_bytes=$item.PrivateMemorySize64; working_set=$item.WorkingSet64;
        handles=$item.HandleCount; threads=$item.Threads.Count; cpu_seconds=$item.TotalProcessorTime.TotalSeconds;
        gdi_objects=[PageDockGuiResources]::GetGuiResources($item.Handle,0);
        user_objects=[PageDockGuiResources]::GetGuiResources($item.Handle,1) }
}
try {
    $env:LOCALAPPDATA = $profileRoot
    $env:WEBVIEW2_USER_DATA_FOLDER = $null
    $env:WEBVIEW2_BROWSER_EXECUTABLE_FOLDER = $null
    $env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = $null
    $arguments = @("-test.run=$TestPattern", '-test.v', "-test.timeout=$($Duration+$Recovery+180)s",
        "-soak-duration=${Duration}s", "-soak-recovery=${Recovery}s", "-soak-pace=$($Pace.ToString([Globalization.CultureInfo]::InvariantCulture))s",
        "-soak-output=`"$Output`"", "-soak-batch=$Batch", "-soak-visits=$Visits")
    if ($Origins) { $arguments += "-soak-origins=$Origins" }
    if ($ClosePageFirst) { $arguments += '-soak-close-page-first' }
    if ($BareWebView) { $arguments += '-soak-bare-webview' }
    if ($BareEval) { $arguments += '-soak-bare-eval' }
    if ($BareTaskbar) { $arguments += '-soak-bare-taskbar' }
    if ($BareBackground) { $arguments += '-soak-bare-background' }
    if ($BareRender) { $arguments += '-soak-bare-render' }
    if ($BareLoading) { $arguments += '-soak-bare-loading' }
    if ($BareBridge) { $arguments += '-soak-bare-bridge' }
    if ($BareDwell -gt 0) { $arguments += "-soak-bare-dwell=$($BareDwell.ToString([Globalization.CultureInfo]::InvariantCulture))s" }
    $process = Start-Process -FilePath $Executable -ArgumentList $arguments -PassThru -WorkingDirectory $Output `
        -RedirectStandardOutput (Join-Path $Output 'test.log') -RedirectStandardError (Join-Path $Output 'stderr.log')
    $null = $process.Handle # Retain the process handle so even a fast exit has a readable exit code.
    $log = [IO.StreamWriter]::new((Join-Path $Output 'process-memory.jsonl'), $false)
    @{ executable=$Executable; pid=$process.Id; duration_s=$Duration; recovery_s=$Recovery; pace_s=$Pace;
       batch=$Batch; visits=$Visits; os=[Environment]::OSVersion.VersionString; architecture=$env:PROCESSOR_ARCHITECTURE;
       test_pattern=$TestPattern; origins=$Origins; close_page_first=$ClosePageFirst.IsPresent;
       bare_webview=$BareWebView.IsPresent; bare_eval=$BareEval.IsPresent; bare_taskbar=$BareTaskbar.IsPresent;
       bare_background=$BareBackground.IsPresent; bare_render=$BareRender.IsPresent;
       bare_loading=$BareLoading.IsPresent; bare_bridge=$BareBridge.IsPresent; bare_dwell_s=$BareDwell;
       mygo_gpu=$env:MYGO_GPU;
       profile=$profileRoot; boundary='real WebViews and notification registry; OS banner transport replaced' } |
        ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $Output 'manifest.json')
    $nextProgress = 30
    do {
        $alive = -not $process.HasExited
        $app = $null
        if ($alive) {
            try { $app = Sample-Process $process }
            catch { if (-not $process.HasExited) { throw }; $alive = $false }
        }
        $children = @()
        foreach ($native in (Owned-WebViews)) {
            $child = Get-Process -Id $native.ProcessId -ErrorAction SilentlyContinue
            if ($child) {
                try { $children += Sample-Process $child }
                catch { if (-not $child.HasExited) { throw } }
                finally { $child.Dispose() }
            }
        }
        $row = [ordered]@{time=[DateTime]::UtcNow.ToString('o'); elapsed_s=$watch.Elapsed.TotalSeconds;
            app=$app; webview_count=$children.Count; webviews=$children}
        $log.WriteLine(($row | ConvertTo-Json -Depth 5 -Compress))
        $log.Flush()
        if ($watch.Elapsed.TotalSeconds -ge $nextProgress) {
            $row | ConvertTo-Json -Depth 5 -Compress | Write-Output
            $nextProgress += 30
        }
        if ($watch.Elapsed.TotalSeconds -gt $Duration+$Recovery+240) { throw 'Independent runner deadline exceeded' }
        if ($alive) { Start-Sleep -Seconds 5 }
    } while ($alive)
    $process.WaitForExit()
    $deadline = [DateTime]::UtcNow.AddSeconds(60)
    while ((Owned-WebViews).Count -gt 0 -and [DateTime]::UtcNow -lt $deadline) { Start-Sleep -Seconds 2 }
    $remaining = (Owned-WebViews).Count
    if ($process.ExitCode -ne 0) { throw "Test process failed: $($process.ExitCode)" }
    if ($TestPattern -eq '^TestDesktopResourceSoak$' -and -not (Test-Path (Join-Path $Output 'completed'))) { throw 'Completion marker missing' }
    if ($remaining -ne 0) { throw "WebView2 processes remain after exit: $remaining" }
    @{result='passed'; elapsed_s=$watch.Elapsed.TotalSeconds; webview_after_exit=$remaining; exit_code=$process.ExitCode} |
        ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $Output 'result.json')
} finally {
    if ($log) { $log.Dispose() }
    if ($process) {
        if (-not $process.HasExited -and $process.Path -eq $Executable) { $process.Kill(); $process.WaitForExit(10000) | Out-Null }
        $process.Dispose()
    }
    foreach ($native in (Owned-WebViews)) {
        $child = Get-Process -Id $native.ProcessId -ErrorAction SilentlyContinue
        if ($child) {
            # Recheck identity after the snapshot to avoid acting on a reused PID.
            if (($child.StartTime.ToUniversalTime() - $native.CreationDate.ToUniversalTime()).Duration().TotalSeconds -lt 1) { $child.Kill() }
            $child.Dispose()
        }
    }
    foreach ($key in $saved.Keys) { [Environment]::SetEnvironmentVariable($key,$saved[$key],'Process') }
}
