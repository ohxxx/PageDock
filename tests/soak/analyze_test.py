#!/usr/bin/env python3
"""[INPUT]: Synthetic measurements with independent process/test clocks.
[OUTPUT]: Checks that process samples share the test clock and slopes retain units.
[POS]: Offline behavior checks for endurance evidence analysis.
[PROTOCOL]: Update tests/soak/AGENTS.md when measurement boundaries change.
"""

import unittest

import analyze


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


if __name__ == "__main__":
    unittest.main()
