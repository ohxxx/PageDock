#!/usr/bin/env python3
"""[INPUT]: Synthetic measurements with independent process/test clocks.
[OUTPUT]: Checks that process samples share the test clock and slopes retain units.
[POS]: Offline behavior checks for endurance evidence analysis.
[PROTOCOL]: Update tests/soak/AGENTS.md when measurement boundaries change.
"""

import unittest

import analyze
import analyze_windows


class MeasurementTests(unittest.TestCase):
    def test_process_samples_use_the_test_clock(self):
        memory = [{"time": "2026-10-19T00:00:02Z", "elapsed_s": 0.5}]
        process = [{"time": "2026-10-19T00:00:03+00:00", "elapsed_s": 3.0}]
        aligned = analyze.align_process_samples(memory, process)
        self.assertEqual(aligned[0]["elapsed_s"], 1.5)
        self.assertEqual(process[0]["elapsed_s"], 3.0)

    def test_slope_is_per_minute_and_missing_values_are_excluded(self):
        rows = [{"elapsed_s": t, "heap": value} for t, value in
                [(0, 10), (30, None), (60, 12), (120, 14)]]
        summary = analyze.describe(rows, "heap", 1)
        self.assertEqual(summary["samples"], 3)
        self.assertEqual(summary["median"], 12)
        self.assertEqual(summary["slope_per_minute"], 2)

    def test_windows_private_memory_stays_separate_from_working_set(self):
        app = {"private_bytes": 10, "working_set": 100, "handles": 4, "threads": 2,
               "gdi_objects": 1, "user_objects": 3}
        rows = [{"elapsed_s": 0, "app": app, "webview_count": 2,
                 "webviews": [{"private_bytes": 2, "working_set": 50},
                              {"private_bytes": 3, "working_set": 60}]},
                {"elapsed_s": 5, "app": None, "webview_count": 0, "webviews": []}]
        values = analyze_windows.flatten(rows)
        self.assertEqual(len(values), 1)
        self.assertEqual(values[0]["app_private"], 10)
        self.assertEqual(values[0]["webview_private"], 5)
        self.assertEqual(values[0]["webview_working_set"], 110)


if __name__ == "__main__":
    unittest.main()
