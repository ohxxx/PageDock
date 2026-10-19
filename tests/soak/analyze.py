#!/usr/bin/env python3
"""[INPUT]: Completed endurance JSONL evidence, manifest, and runner result.
[OUTPUT]: Windowed memory statistics, a time-series PNG, and an evidence summary.
[POS]: Offline analysis only; does not launch apps or alter collected samples.
[PROTOCOL]: Update tests/soak/AGENTS.md when measurement interpretation changes.
"""

import argparse
import datetime
import json
from pathlib import Path
import statistics


MIB = 1024**2


def read_rows(path):
    return [json.loads(line) for line in path.read_text().splitlines() if line.strip()]


def align_process_samples(memory, rss):
    # LaunchServices and the Go test begin at different times. Keep raw evidence intact.
    origin = datetime.datetime.fromisoformat(memory[0]["time"].replace("Z", "+00:00")) - datetime.timedelta(seconds=memory[0]["elapsed_s"])
    return [{**row, "elapsed_s": (datetime.datetime.fromisoformat(row["time"].replace("Z", "+00:00")) - origin).total_seconds()}
            for row in rss]


def describe(rows, field, scale):
    values = [(row["elapsed_s"] / 60, row[field] / scale) for row in rows if row[field] is not None]
    if not values:
        return None
    times, samples = zip(*values)
    mean_time, mean_value = statistics.mean(times), statistics.mean(samples)
    denominator = sum((time - mean_time)**2 for time in times)
    slope = sum((time - mean_time) * (value - mean_value) for time, value in values) / denominator if denominator else None
    return {"samples": len(samples), "first": samples[0], "last": samples[-1],
            "min": min(samples), "max": max(samples), "median": statistics.median(samples),
            "slope_per_minute": slope}


def summarize(memory, rss, manifest, result):
    checkpoints = [row for row in memory if row["post_gc"]]
    loaded = [row for row in checkpoints if row["phase"] in ("loaded", "loaded_final")]
    final_load = next(row for row in checkpoints if row["phase"] == "loaded_final")
    idle = next(row for row in checkpoints if row["phase"] == "idle_final")
    active_end = final_load["elapsed_s"]
    intervals = {"first_5_minutes": (0, min(300, active_end)),
                 "warm_load": (min(300, active_end / 2), active_end),
                 "last_5_minutes_load": (max(0, active_end - 300), active_end),
                 "idle_recovery": (active_end, idle["elapsed_s"])}
    windows = {}
    for name, (start, end) in intervals.items():
        native = [row for row in rss if start <= row["elapsed_s"] <= end and not row["app_exited"] and row["app_rss_kib"] > 0]
        heap = [row for row in checkpoints if start <= row["elapsed_s"] <= end and row["phase"] != "forced_scavenge"]
        windows[name] = {"range_s": [start, end],
                         "app_rss_mib": describe(native, "app_rss_kib", 1024),
                         "app_footprint_mib": describe(native, "app_footprint", MIB),
                         "webkit_rss_mib": describe(native, "webkit_rss_kib", 1024),
                         "webkit_footprint_mib": describe(native, "webkit_footprint", MIB),
                         "heap_after_gc_mib": describe(heap, "heap_alloc", MIB)}
    return {"manifest": manifest, "result": result, "active_seconds": active_end,
            "recovery_seconds": idle["elapsed_s"] - active_end,
            "counts": {name: final_load[name] for name in ("cycles", "notifications", "navigations", "clicks")},
            "cold": checkpoints[0], "final_load": final_load, "idle_final": idle,
            "forced_scavenge": next(row for row in memory if row["phase"] == "forced_scavenge"),
            "load_heap_range_mib": [min(row["heap_alloc"] for row in loaded) / MIB, max(row["heap_alloc"] for row in loaded) / MIB],
            "max_observed_webkit_processes": max(row["webkit_count"] for row in rss),
            "windows": windows}


def plot(output, memory, rss, summary):
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    plt.rcParams.update({"font.size": 10, "axes.spines.top": False, "axes.spines.right": False})
    figure, panels = plt.subplots(2, 2, figsize=(13, 8), constrained_layout=True)
    native = [row for row in rss if not row["app_exited"] and row["app_rss_kib"] > 0
              and 0 <= row["elapsed_s"] <= summary["idle_final"]["elapsed_s"]]
    checkpoint = [row for row in memory if row["post_gc"] and row["phase"] != "forced_scavenge"]
    for axis, title, rss_key, footprint_key in (
        (panels[0, 0], "Application process", "app_rss_kib", "app_footprint"),
        (panels[0, 1], "Owned WebKit processes (sum)", "webkit_rss_kib", "webkit_footprint"),
    ):
        axis.plot([row["elapsed_s"] / 60 for row in native], [row[rss_key] / 1024 for row in native], label="RSS", color="#24719f", lw=1.2)
        footprints = [row for row in native if row[footprint_key] is not None]
        axis.plot([row["elapsed_s"] / 60 for row in footprints], [row[footprint_key] / MIB for row in footprints], label="Physical footprint", color="#d27d2d", lw=1.2)
        axis.set(title=title, ylabel="MiB")
    times = [row["elapsed_s"] / 60 for row in checkpoint]
    panels[1, 0].plot(times, [row["heap_alloc"] / MIB for row in checkpoint], color="#278361", label="Live Go heap after GC", marker=".", ms=3)
    panels[1, 0].set(title="Go heap (uniform GC checkpoints)", ylabel="MiB")
    for field, label, color in (("native_records", "Native notification records", "#24719f"),
                                ("active_callbacks", "Page callbacks after close", "#c45050")):
        panels[1, 1].plot(times, [row[field] for row in checkpoint], label=label, color=color, lw=1.5)
    panels[1, 1].set(title="Notification lifetime", ylabel="Records")
    for axis in panels.flat:
        axis.axvspan(summary["active_seconds"] / 60, summary["idle_final"]["elapsed_s"] / 60, color="#e3ece5", alpha=.6, label="Idle recovery")
        axis.set_xlabel("Elapsed minutes")
        axis.set_xlim(left=0)
        axis.set_ylim(bottom=0)
        axis.grid(alpha=.18)
        axis.legend(loc="best", fontsize=8)
    count = summary["counts"]
    mode = {True: "development", False: "production options", None: "build mode unrecorded"}[summary["manifest"].get("development")]
    figure.suptitle(f"PageDock macOS endurance ({mode}) | {count['notifications']:,} notifications, {count['navigations']:,} navigations, {count['cycles']:,} windows\n"
                   "Real WebViews + notification registry; system banner transport replaced. Forced scavenging excluded.", fontsize=13)
    figure.savefig(output / "memory-trend.png", dpi=160)
    plt.close(figure)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("evidence", type=Path)
    args = parser.parse_args()
    output = args.evidence.resolve()
    memory, rss = read_rows(output / "go-memory.jsonl"), read_rows(output / "rss.jsonl")
    rss = align_process_samples(memory, rss)
    manifest = json.loads((output / "manifest.json").read_text())
    result = json.loads((output / "result.json").read_text())
    if result["result"] != "passed":
        raise RuntimeError("Analyze only a complete successful workload; failed logs remain raw evidence")
    summary = summarize(memory, rss, manifest, result)
    (output / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    plot(output, memory, rss, summary)
    print(json.dumps({"counts": summary["counts"], "active_seconds": summary["active_seconds"],
                      "recovery_seconds": summary["recovery_seconds"], "windows": summary["windows"]}, indent=2))


if __name__ == "__main__":
    main()
