#!/usr/bin/env python3
"""[INPUT]: Disposable local process fixtures and the endurance runner's cleanup routine.
[OUTPUT]: Behavior checks for exact-bundle isolation, repeat cleanup, and TERM escalation.
[POS]: macOS runner safety tests; does not launch desktop apps or touch existing processes.
[PROTOCOL]: Update tests/soak/AGENTS.md when runner ownership or cleanup changes.
"""

import importlib.util
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import unittest


spec = importlib.util.spec_from_file_location("soak_runner", Path(__file__).with_name("run.py"))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


@unittest.skipUnless(sys.platform == "darwin", "macOS process-name semantics")
class CleanupTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="pagedock-soak-cleanup-")
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name).resolve()

    def start(self, name, program, *arguments):
        bundle = self.root / name
        executable = bundle / "Contents/MacOS/PageDockSoak"
        executable.parent.mkdir(parents=True)
        shutil.copy(program, executable)
        process = subprocess.Popen([str(executable), *arguments], stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)

        def cleanup():
            if process.poll() is None:
                process.kill()
            process.wait(timeout=10)
            process.stdin.close()
            process.stdout.close()

        self.addCleanup(cleanup)
        return bundle, process

    def test_only_the_exact_bundle_is_stopped(self):
        bundle, owned = self.start("test.app", "/bin/sleep", "60")
        _, unrelated = self.start("test.app-other", "/bin/sleep", "60")
        runner.stop_test_bundle(bundle)
        self.assertEqual(owned.wait(timeout=10), -signal.SIGTERM)
        self.assertIsNone(unrelated.poll())
        runner.stop_test_bundle(bundle)
        self.assertIsNone(unrelated.poll())

    def test_a_stuck_owned_process_is_killed(self):
        bundle, owned = self.start("stuck.app", "/bin/zsh", "-c", "trap '' TERM; print ready; read -t 60")
        self.assertEqual(owned.stdout.readline().strip(), "ready")
        runner.stop_test_bundle(bundle)
        self.assertEqual(owned.wait(timeout=10), -signal.SIGKILL)


if __name__ == "__main__":
    unittest.main()
