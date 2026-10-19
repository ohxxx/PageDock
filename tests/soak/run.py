#!/usr/bin/env python3
"""[INPUT]: Explicit duration/recovery flags and the repository's pinned MyGo/test overlay.
[OUTPUT]: An isolated macOS test bundle/run or Windows test EXE, with explicit transport boundaries.
[POS]: Opt-in endurance runner; only the notification presentation transport is replaced.
[PROTOCOL]: Update tests/soak/AGENTS.md, tests/AGENTS.md, and the owning test headers.
"""

import argparse
import ctypes
import datetime
import hashlib
import json
import os
from pathlib import Path
import plistlib
import signal
import subprocess
import sys
import time
import uuid


ROOT = Path(__file__).resolve().parents[2]
MYGO_MODULE = "github.com/egoist/mygo"
LSREGISTER = Path("/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister")


def run(command, **kwargs):
    return subprocess.run(command, cwd=ROOT, check=True, **kwargs)


def replace_body(source, signature, body):
    if source.count(signature) != 1:
        raise RuntimeError(f"Pinned notification transport changed: {signature}")
    start = source.index(signature) + len(signature)
    end = source.index("\n}", start)
    return source[:start] + "\n" + body + source[end:]


def prepare(output, race, development, platform="darwin", arch=None):
    modfile = run(["go", "run", "./tools/mygo-patch"], capture_output=True, text=True).stdout.strip()
    base_overlay = run(["go", "run", "./tools/test-overlay"], capture_output=True, text=True).stdout.strip()
    module = json.loads(run(["go", "list", f"-modfile={modfile}", "-m", "-json", MYGO_MODULE], capture_output=True, text=True).stdout)
    if module["Version"] != "v0.5.1":
        raise RuntimeError("Review the observation overlay before using a different MyGo version")
    dependency = Path(module["Dir"])
    target = dependency / ("internal/darwin/notify.go" if platform == "darwin" else "internal/windows/tray.go")
    original = target.read_text()
    source = original
    replacements = [
        ("func (b *Backend) NotificationsSupported() bool {", "\treturn true"),
        ("func (b *Backend) ShowNotification(n *platform.Notification, done func(error)) {", "\tdone(nil)"),
        ("func (b *Backend) RemoveNotification(" + ("ident" if platform == "darwin" else "id") + " string) {", ""),
        ("func (b *Backend) RemoveAllNotifications() {", ""),
    ]
    if platform == "darwin":
        replacements.append(("func (b *Backend) attachNotificationDelegate() {", ""))
    else:
        # Windows already reports support in a one-line method.
        replacements = replacements[1:]
    for signature, body in replacements:
        source = replace_body(source, signature, body)
    transport = output / "notify_transport.go"
    transport.write_text(source)
    probe = output / "mygo_probe.go"
    probe.write_text('''//go:build pagedock_soak

package mygo

// These observation hooks exist only in the opt-in test overlay.
func PageDockSoakNativeCount() int {
    notifications.Lock()
    defer notifications.Unlock()
    return len(notifications.byID)
}

func PageDockSoakClick(id string) {
    onMain(func() { notificationClicked(id) })
}
''')
    overlay = json.loads(Path(base_overlay).read_text())
    overlay["Replace"][str(target)] = str(transport)
    overlay["Replace"][str(dependency / "pagedock_soak_probe.go")] = str(probe)
    overlay_path = output / "overlay.json"
    overlay_path.write_text(json.dumps(overlay, indent=2))
    bundle = output / "PageDockSoak.app"
    executable_dir = bundle / "Contents/MacOS"
    if platform == "darwin":
        executable_dir.mkdir(parents=True)
    identifier = "local.pagedock.soak." + uuid.uuid4().hex
    if platform == "darwin":
        with (bundle / "Contents/Info.plist").open("wb") as file:
            plistlib.dump({"CFBundleIdentifier": identifier, "CFBundleName": "PageDockSoak", "CFBundleExecutable": "PageDockSoak",
                          "CFBundlePackageType": "APPL", "CFBundleVersion": "1", "NSHighResolutionCapable": True}, file)
    executable = executable_dir / "PageDockSoak" if platform == "darwin" else output / "PageDockSoak.exe"
    tags = "desktop,pagedock_soak" if development else "desktop,pagedock_soak,mygo_noinspector"
    command = ["go", "test", f"-modfile={modfile}", f"-overlay={overlay_path}", "-tags", tags, "-c", "-o", str(executable)]
    if not development:
        command += ["-ldflags", "-X github.com/egoist/mygo.production=1"]
    if race:
        command.append("-race")
    command.append("./internal")
    with (output / "build.log").open("w") as log:
        environment = os.environ.copy()
        if platform == "windows":
            environment.update(GOOS="windows", GOARCH=arch, CGO_ENABLED="0")
        run(command, stdout=log, stderr=subprocess.STDOUT, env=environment)
        if platform == "darwin":
            run(["codesign", "--force", "--sign", "-", str(bundle)], stdout=log, stderr=subprocess.STDOUT)
            run(["codesign", "--verify", "--deep", "--strict", str(bundle)], stdout=log, stderr=subprocess.STDOUT)
    return bundle if platform == "darwin" else executable, {"platform":platform,"arch":arch,"bundle_id": identifier if platform == "darwin" else None,
                    "race": race, "development": development, "module": module["Version"], "transport_original_sha256": hashlib.sha256(original.encode()).hexdigest(),
                    "transport_overlay_sha256": hashlib.sha256(source.encode()).hexdigest(), "boundary": "real WebViews and MyGo notifications.byID; OS banner transport replaced"}


def responsibility_api():
    function = ctypes.CDLL("/usr/lib/libSystem.B.dylib").responsibility_get_pid_responsible_for_pid
    function.argtypes = [ctypes.c_int]
    function.restype = ctypes.c_int
    return function


class RUsageInfo(ctypes.Structure):
    # RUSAGE_INFO_V0 from the macOS SDK's sys/resource.h.
    _fields_ = [("uuid", ctypes.c_uint8 * 16)] + [
        (name, ctypes.c_uint64) for name in (
            "user_time", "system_time", "pkg_idle_wkups", "interrupt_wkups", "pageins",
            "wired_size", "resident_size", "phys_footprint", "proc_start_abstime", "proc_exit_abstime",
        )
    ]


def footprint_api():
    function = ctypes.CDLL("/usr/lib/libproc.dylib").proc_pid_rusage
    function.argtypes = [ctypes.c_int, ctypes.c_int, ctypes.c_void_p]
    function.restype = ctypes.c_int
    return function


def process_sample(pid, responsible, footprint, elapsed, exited=False):
    # Read names only, never command arguments/environment from other applications.
    table = run(["ps", "-axo", "pid=,ppid=,rss=,%cpu=,comm="], capture_output=True, text=True).stdout
    processes = []
    for line in table.splitlines():
        fields = line.split(None, 4)
        if len(fields) != 5:
            continue
        child, parent, rss, cpu, name = fields
        child = int(child)
        if child != pid and "com.apple.WebKit." not in name:
            continue
        owner = responsible(child)
        if child == pid or owner == pid:
            usage = RUsageInfo()
            footprint_bytes = usage.phys_footprint if footprint(child, 0, ctypes.byref(usage)) == 0 else None
            processes.append({"pid": child, "ppid": int(parent), "responsible_pid": owner,
                              "rss_kib": int(rss), "phys_footprint": footprint_bytes,
                              "cpu_percent": float(cpu), "name": name})
    app = [row for row in processes if row["pid"] == pid]
    children = [row for row in processes if row["pid"] != pid]
    return {"time": datetime.datetime.now(datetime.timezone.utc).isoformat(), "elapsed_s": elapsed, "app_exited": exited,
            "app_rss_kib": sum(row["rss_kib"] for row in app), "webkit_rss_kib": sum(row["rss_kib"] for row in children),
            "app_footprint": sum(row["phys_footprint"] for row in app) if all(row["phys_footprint"] is not None for row in app) else None,
            "webkit_footprint": sum(row["phys_footprint"] for row in children) if all(row["phys_footprint"] is not None for row in children) else None,
            "webkit_count": len(children), "processes": processes}


def monitor(bundle, output, args):
    stdout, stderr = output / "test.log", output / "stderr.log"
    stdout.touch()
    stderr.touch()
    command = ["open", "-n", "-g", "-W", "--stdout", str(stdout), "--stderr", str(stderr)]
    if args.malloc_stack_logging:
        # WebKit's bmalloc needs system allocation to expose individual owner stacks.
        command += ["--env", "MallocStackLogging=1", "--env", "Malloc=1"]
    command += [str(bundle), "--args",
               "-test.run=^TestDesktopResourceSoak$", "-test.v", f"-test.timeout={args.duration+args.recovery+180}s",
               f"-soak-duration={args.duration}s", f"-soak-recovery={args.recovery}s", f"-soak-pace={args.pace}s", f"-soak-output={output}",
               f"-soak-batch={args.batch}", f"-soak-visits={args.visits}"]
    started = time.monotonic()
    process = subprocess.Popen(command, cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    pid_file = output / "pid"
    while not pid_file.exists():
        if process.poll() is not None:
            raise RuntimeError(f"Test app exited before startup: {process.communicate()}\n{stderr.read_text()}")
        if time.monotonic()-started > 60:
            raise RuntimeError("Test app did not write its PID within 60 seconds")
        time.sleep(0.2)
    pid = int(pid_file.read_text())
    responsible = responsibility_api()
    footprint = footprint_api()
    if responsible(pid) != pid:
        raise RuntimeError(f"Cannot attribute WebKit processes: test PID {pid} belongs to {responsible(pid)}")
    print(json.dumps({"event": "started", "pid": pid, "duration_s": args.duration, "recovery_s": args.recovery}), flush=True)
    last_progress = 0
    with (output / "rss.jsonl").open("w") as log:
        while process.poll() is None:
            elapsed = time.monotonic()-started
            row = process_sample(pid, responsible, footprint, elapsed)
            log.write(json.dumps(row)+"\n")
            log.flush()
            if elapsed-last_progress >= 30:
                print(json.dumps({"event": "memory", "elapsed_s": round(elapsed), "app_mib": round(row["app_rss_kib"]/1024,2),
                                  "webkit_mib": round(row["webkit_rss_kib"]/1024,2), "webkit_processes": row["webkit_count"]}), flush=True)
                last_progress = elapsed
            if elapsed > args.duration+args.recovery+240:
                raise RuntimeError("Endurance runner exceeded its independent deadline")
            time.sleep(5)
        launch_stdout, launch_stderr = process.communicate()
        if process.returncode:
            raise RuntimeError(f"LaunchServices wait failed: {launch_stdout} {launch_stderr}")
        deadline = time.monotonic()+60
        while True:
            row = process_sample(pid, responsible, footprint, time.monotonic()-started, True)
            log.write(json.dumps(row)+"\n")
            log.flush()
            if row["webkit_count"] == 0:
                break
            if time.monotonic() >= deadline:
                raise RuntimeError(f"WebKit processes remain after exit: {row['processes']}")
            time.sleep(5)
    if not (output / "completed").exists() or "--- FAIL:" in stdout.read_text() or "WARNING: DATA RACE" in stderr.read_text():
        raise RuntimeError(f"Endurance assertions failed; inspect {stdout} and {stderr}")
    return {"pid": pid, "elapsed_s": time.monotonic()-started, "webkit_after_exit": row["webkit_count"], "result": "passed"}


def stop_test_bundle(bundle):
    executable = str(bundle / "Contents/MacOS/PageDockSoak")

    def owned_pids():
        table = run(["ps", "-axo", "pid=,comm="], capture_output=True, text=True).stdout
        return [int(parts[0]) for line in table.splitlines()
                if len(parts := line.split(None, 1)) == 2 and parts[1] == executable]

    # Recheck the exact executable path before every signal, including after a timeout.
    for sig in (signal.SIGTERM, signal.SIGKILL):
        for pid in owned_pids():
            try:
                os.kill(pid, sig)
            except ProcessLookupError:
                pass
        deadline = time.monotonic() + 5
        while owned_pids() and time.monotonic() < deadline:
            time.sleep(0.2)
    if owned_pids():
        raise RuntimeError("The isolated test app survived cleanup")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--duration", type=int, default=1800, help="active seconds")
    parser.add_argument("--recovery", type=int, default=300, help="idle seconds")
    parser.add_argument("--pace", type=float, default=3, help="minimum seconds between window cycles")
    parser.add_argument("--batch", type=int, default=128, help="notifications per page; zero isolates navigation/window churn")
    parser.add_argument("--visits", type=int, default=3, help="trusted page visits per window")
    parser.add_argument("--race", action="store_true", help="short correctness run; not a production RSS baseline")
    parser.add_argument("--development", action="store_true", help="retain MyGo developer tools for comparison with production defaults")
    parser.add_argument("--malloc-stack-logging", action="store_true", help="record native allocation stacks using system malloc in this test process only")
    parser.add_argument("--windows-build", choices=["arm64", "amd64"], help="build an isolated Windows test EXE; run it with windows.ps1 on Windows")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if sys.platform != "darwin" or min(args.duration,args.recovery,args.pace,args.visits) <= 0 or args.batch < 0 or 0 < args.batch < 4:
        parser.error("positive durations and macOS are required")
    output = args.output.resolve()
    if not output.is_relative_to(ROOT / ".scratch"):
        parser.error("evidence must stay under the repository's .scratch directory")
    output.mkdir(parents=True, exist_ok=False)
    if args.windows_build and (args.race or args.malloc_stack_logging):
        parser.error("Windows cross-builds do not support --race or --malloc-stack-logging")
    bundle, metadata = prepare(output,args.race,args.development,"windows" if args.windows_build else "darwin",args.windows_build)
    metadata.update({"duration_s":args.duration,"recovery_s":args.recovery,"pace_s":args.pace,"batch":args.batch,"visits":args.visits,
                     "malloc_stack_logging":args.malloc_stack_logging})
    (output / "manifest.json").write_text(json.dumps(metadata,indent=2))
    if args.windows_build:
        print(json.dumps({"executable":str(bundle),"result":"built; Windows execution required"}))
        return
    try:
        result = monitor(bundle,output,args)
        (output / "result.json").write_text(json.dumps(result,indent=2))
        print(json.dumps(result),flush=True)
    finally:
        stop_test_bundle(bundle)
        if LSREGISTER.exists():
            with (output / "unregister.log").open("w") as log:
                run([str(LSREGISTER),"-u",str(bundle)],stdout=log,stderr=subprocess.STDOUT)


if __name__ == "__main__":
    main()
