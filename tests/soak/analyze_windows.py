#!/usr/bin/env python3
"""[INPUT]: Completed Windows endurance evidence from windows.ps1.
[OUTPUT]: Interval statistics and a memory/handle/process chart, preserving native-growth findings.
[POS]: Offline analysis; a passed behavior workload does not imply leak-free native resources.
[PROTOCOL]: Update tests/soak/AGENTS.md with measurement boundaries.
"""

import argparse
import json
from pathlib import Path

from analyze import MIB, align_process_samples, describe, read_rows


def flatten(rows):
    return [{"elapsed_s": row["elapsed_s"],
             "app_private": row["app"]["private_bytes"],
             "app_working_set": row["app"]["working_set"],
             "app_handles": row["app"]["handles"],
             "app_threads": row["app"]["threads"],
             "app_gdi": row["app"]["gdi_objects"],
             "app_user": row["app"]["user_objects"],
             "webview_private": sum(child["private_bytes"] for child in row["webviews"]),
             "webview_working_set": sum(child["working_set"] for child in row["webviews"]),
             "webview_count": row["webview_count"]}
            for row in rows if row["app"] is not None]


def summarize(memory, native, manifest, result):
    checkpoints = [row for row in memory if row["post_gc"] and row["phase"] != "forced_scavenge"]
    final_load = next(row for row in checkpoints if row["phase"] == "loaded_final")
    idle = next(row for row in checkpoints if row["phase"] == "idle_final")
    end = final_load["elapsed_s"]
    windows = {}
    for name, start, stop in (("warm_load", min(300, end / 2), end),
                              ("last_5_minutes_load", max(0, end - 300), end),
                              ("idle_recovery", end, idle["elapsed_s"])):
        rows = [row for row in native if start <= row["elapsed_s"] <= stop]
        stats = {field: describe(rows, field, MIB) for field in
                 ("app_private", "app_working_set", "webview_private", "webview_working_set")}
        stats.update({field: describe(rows, field, 1) for field in
                      ("app_handles", "app_threads", "app_gdi", "app_user", "webview_count")})
        heap = [row for row in checkpoints if start <= row["elapsed_s"] <= stop]
        stats["heap_after_gc_mib"] = describe(heap, "heap_alloc", MIB)
        windows[name] = {"range_s": [start, stop], **stats}
    return {"manifest": manifest, "behavior_result": result,
            "active_seconds": end, "recovery_seconds": idle["elapsed_s"] - end,
            "counts": {key: final_load[key] for key in ("cycles", "navigations", "notifications", "clicks")},
            "final_load": final_load, "idle_final": idle, "windows": windows,
            "interpretation": "Native memory/handle trends require review even when behavior assertions pass. Working sets include shared mappings; do not sum them as unique RAM."}


def plot(output, memory, native, summary):
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    idle = summary["idle_final"]["elapsed_s"]
    rows = [row for row in native if 0 <= row["elapsed_s"] <= idle]
    checkpoints = [row for row in memory if row["post_gc"] and row["phase"] != "forced_scavenge"]
    figure, panels = plt.subplots(2, 3, figsize=(15, 8), constrained_layout=True)
    times = [row["elapsed_s"] / 60 for row in rows]
    for axis, title, fields, scale, unit in (
        (panels[0, 0], "App memory", ("app_private", "app_working_set"), MIB, "MiB"),
        (panels[0, 1], "WebView2 private memory", ("webview_private",), MIB, "MiB"),
        (panels[0, 2], "App handles", ("app_handles",), 1, "Handles"),
        (panels[1, 2], "Owned WebView2 processes", ("webview_count",), 1, "Processes"),
    ):
        for field in fields:
            axis.plot(times, [row[field] / scale for row in rows], label=field, lw=1.2)
        axis.set(title=title, ylabel=unit)
    times = [row["elapsed_s"] / 60 for row in checkpoints]
    panels[1, 0].plot(times, [row["heap_alloc"] / MIB for row in checkpoints], label="Go heap after GC")
    panels[1, 0].set(title="Live Go heap", ylabel="MiB")
    panels[1, 1].plot(times, [row["native_records"] for row in checkpoints], label="Notification records")
    panels[1, 1].set(title="Bounded notifications", ylabel="Records")
    for axis in panels.flat:
        axis.axvspan(summary["active_seconds"] / 60, idle / 60, color="#e3ece5", alpha=.6, label="Idle recovery")
        axis.set(xlabel="Elapsed minutes", xlim=(0, idle / 60), ylim=(0, None))
        axis.grid(alpha=.18)
        axis.legend(fontsize=8)
    counts = summary["counts"]
    figure.suptitle(f"PageDock Windows endurance | {counts['cycles']:,} windows, {counts['notifications']:,} notifications\n"
                   "Real WebView2; system banner transport replaced. Forced scavenging excluded.")
    figure.savefig(output / "memory-trend.png", dpi=160)
    plt.close(figure)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("evidence", type=Path)
    output = parser.parse_args().evidence.resolve()
    memory = read_rows(output / "go-memory.jsonl")
    native = flatten(align_process_samples(memory, read_rows(output / "process-memory.jsonl")))
    manifest = json.loads((output / "manifest.json").read_text(encoding="utf-8-sig"))
    result = json.loads((output / "result.json").read_text(encoding="utf-8-sig"))
    if result["result"] != "passed":
        raise RuntimeError("A completed behavior workload is required; retain failed logs separately")
    summary = summarize(memory, native, manifest, result)
    (output / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    plot(output, memory, native, summary)
    print(json.dumps({"counts": summary["counts"], "windows": summary["windows"]}, indent=2))


if __name__ == "__main__":
    main()
