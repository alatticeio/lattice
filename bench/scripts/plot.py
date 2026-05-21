#!/usr/bin/env python3
"""bench/scripts/plot.py

Generates bar charts from E2E benchmark JSON result files.
Usage: python3 plot.py [results_dir]

Requires: pip install matplotlib
"""
import json
import sys
from pathlib import Path


def load_summaries(results_dir: Path) -> list[dict]:
    summaries = []
    for f in sorted(results_dir.glob("summary_*.json")):
        with f.open() as fh:
            summaries.append(json.load(fh))
    return summaries


def plot_throughput(summaries: list[dict], out_path: Path) -> None:
    try:
        import matplotlib.pyplot as plt
    except ImportError:
        print("matplotlib not found. Install with: pip install matplotlib")
        return

    if not summaries:
        print("No summary_*.json files found in results dir.")
        return

    latest = summaries[-1]
    labels = ["TCP", "UDP"]
    bare = [latest.get("tcp_bare_mbps", 0), latest.get("udp_bare_mbps", 0)]
    overlay = [latest.get("tcp_overlay_mbps", 0), latest.get("udp_overlay_mbps", 0)]

    x = range(len(labels))
    width = 0.35

    fig, ax = plt.subplots(figsize=(6, 4))
    ax.bar([i - width / 2 for i in x], bare, width, label="Bare Metal")
    ax.bar([i + width / 2 for i in x], overlay, width, label="Overlay")

    ax.set_ylabel("Throughput (Mbps)")
    ax.set_title(f"Lattice Throughput — {latest.get('timestamp', 'unknown')}")
    ax.set_xticks(list(x))
    ax.set_xticklabels(labels)
    ax.legend()

    fig.tight_layout()
    fig.savefig(out_path)
    print(f"Chart saved to {out_path}")


def main() -> None:
    results_dir = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(__file__).parent.parent / "results"
    summaries = load_summaries(results_dir)
    plot_throughput(summaries, results_dir / "throughput.png")


if __name__ == "__main__":
    main()
