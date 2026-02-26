"""Data analysis skill for Skillbox.

Reads JSON or CSV input, computes descriptive statistics,
and outputs a summary with an optional chart.
"""

import csv
import io
import json
import math
import os
import sys


def read_input():
    """Read input from SANDBOX_INPUT env var or /sandbox/input.json."""
    raw = os.environ.get("SANDBOX_INPUT", "")
    if raw:
        return json.loads(raw)
    input_path = "/sandbox/input.json"
    if os.path.exists(input_path):
        with open(input_path) as f:
            return json.load(f)
    return {}


def parse_csv_text(csv_text):
    """Parse CSV text into list of dicts."""
    reader = csv.DictReader(io.StringIO(csv_text))
    return list(reader)


def compute_stats(values):
    """Compute descriptive statistics for a list of numeric values."""
    n = len(values)
    if n == 0:
        return {"count": 0}

    mean = sum(values) / n
    sorted_vals = sorted(values)
    median = sorted_vals[n // 2] if n % 2 else (sorted_vals[n // 2 - 1] + sorted_vals[n // 2]) / 2

    variance = sum((x - mean) ** 2 for x in values) / n if n > 1 else 0
    std_dev = math.sqrt(variance)

    return {
        "count": n,
        "mean": round(mean, 4),
        "median": round(median, 4),
        "std_dev": round(std_dev, 4),
        "min": round(min(values), 4),
        "max": round(max(values), 4),
        "sum": round(sum(values), 4),
    }


def analyze(data):
    """Analyze a list of records and return per-column statistics."""
    if not data:
        return {"error": "No data provided", "columns": {}}

    columns = {}
    for record in data:
        for key, value in record.items():
            if key not in columns:
                columns[key] = []
            columns[key].append(value)

    stats = {}
    numeric_columns = {}

    for col, values in columns.items():
        # Try to parse as numbers
        numeric_values = []
        for v in values:
            try:
                numeric_values.append(float(v))
            except (ValueError, TypeError):
                pass

        if len(numeric_values) > len(values) * 0.5:
            stats[col] = compute_stats(numeric_values)
            stats[col]["type"] = "numeric"
            numeric_columns[col] = numeric_values
        else:
            unique = set(str(v) for v in values)
            stats[col] = {
                "count": len(values),
                "unique": len(unique),
                "type": "categorical",
                "top_values": list(unique)[:10],
            }

    return {
        "row_count": len(data),
        "column_count": len(columns),
        "columns": stats,
        "numeric_columns": list(numeric_columns.keys()),
    }


def write_chart(stats, files_dir):
    """Write a simple text-based chart as a file artifact."""
    numeric_cols = stats.get("numeric_columns", [])
    if not numeric_cols:
        return

    chart_lines = ["# Summary Statistics Chart", ""]
    for col in numeric_cols:
        col_stats = stats["columns"][col]
        bar_len = min(int(col_stats["mean"]), 50)
        bar = "#" * max(bar_len, 1)
        chart_lines.append(f"{col:20s} | {bar} (mean={col_stats['mean']})")

    chart_path = os.path.join(files_dir, "summary.txt")
    with open(chart_path, "w") as f:
        f.write("\n".join(chart_lines))

    print(f"Chart written to {chart_path}")


def main():
    input_data = read_input()

    # Parse data from input
    if "csv" in input_data:
        data = parse_csv_text(input_data["csv"])
    elif "data" in input_data:
        data = input_data["data"]
    else:
        data = []

    # Run analysis
    result = analyze(data)

    # Write output
    output_path = os.environ.get("SANDBOX_OUTPUT", "/sandbox/out/output.json")
    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with open(output_path, "w") as f:
        json.dump(result, f, indent=2)

    print(f"Analysis complete: {result['row_count']} rows, {result['column_count']} columns")

    # Write chart artifact
    files_dir = os.environ.get("SANDBOX_FILES_DIR", "/sandbox/out/files/")
    os.makedirs(files_dir, exist_ok=True)
    write_chart(result, files_dir)


if __name__ == "__main__":
    main()
