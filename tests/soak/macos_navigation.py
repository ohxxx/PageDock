#!/usr/bin/env python3
"""[INPUT]: Standalone native navigation probe and a new evidence directory.
[OUTPUT]: Observed loopback requests, process samples, and stacks on timeout.
[POS]: macOS navigation diagnostic; no installed app or persistent browser data.
[PROTOCOL]: Update tests/soak/AGENTS.md when measurements or isolation change.
"""
import argparse
import http.server
import json
from pathlib import Path
import subprocess
import threading
import time


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("executable", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--cycles", type=int, default=120)
    parser.add_argument("--separate", action="store_true")
    parser.add_argument("--no-animations", action="store_true")
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=False)
    lock = threading.Lock()
    requests = (args.output / "requests.jsonl").open("w")

    class Handler(http.server.BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def do_GET(self):
            body = b"<!doctype html><title>Isolated navigation probe</title><p>ready</p>"
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Cache-Control", "no-store")
            self.send_header("Connection", "close")
            self.end_headers()
            self.wfile.write(body)
            with lock:
                requests.write(json.dumps({"time": time.time(), "port": self.server.server_port, "path": self.path}) + "\n")
                requests.flush()

        def log_message(self, *unused):
            pass

    servers = [http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler) for _ in range(2)]
    for server in servers:
        threading.Thread(target=server.serve_forever, daemon=True).start()
    command = [str(args.executable.resolve()), *[f"http://127.0.0.1:{s.server_port}" for s in servers], str(args.cycles), "production"]
    command += ["separate" if args.separate else "shared", "no-animations" if args.no_animations else "animations"]
    try:
        with (args.output / "native.log").open("w") as log:
            process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
            (args.output / "pid").write_text(str(process.pid))
            try:
                for line in process.stdout:
                    log.write(line)
                    log.flush()
                    if "cycles=" in line or "timeout" in line:
                        with (args.output / "process.log").open("a") as snapshot:
                            subprocess.run(["ps", "-p", str(process.pid), "-o", "pid=,rss=,%cpu=,etime="], stdout=snapshot, check=False)
                    if "timeout" in line:
                        for name, command in [
                            ("descriptors.txt", ["lsof", "-nP", "-p", str(process.pid)]),
                            ("sample.txt", ["sample", str(process.pid), "1", "-file", str(args.output / "stacks.txt")]),
                        ]:
                            with (args.output / name).open("w") as snapshot:
                                subprocess.run(command, stdout=snapshot, stderr=subprocess.STDOUT, timeout=5, check=False)
                code = process.wait(timeout=5)
            finally:
                if process.poll() is None:
                    process.terminate()
                    process.wait(timeout=10)
        (args.output / "result.json").write_text(json.dumps({"exit_code": code, "cycles": args.cycles}))
        print(json.dumps({"exit_code": code, "output": str(args.output)}), flush=True)
        return code
    finally:
        for server in servers:
            server.shutdown()
            server.server_close()
        requests.close()


if __name__ == "__main__":
    raise SystemExit(main())
