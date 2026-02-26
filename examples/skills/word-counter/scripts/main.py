"""Word counter skill for Skillbox.

Counts word frequencies in input text and returns the top N most common words.
"""

import json
import os
import re
from collections import Counter


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


def main():
    input_data = read_input()

    text = input_data.get("text", "")
    top_n = input_data.get("top_n", 10)

    # Tokenize and count
    words = re.findall(r"\b[a-z]+\b", text.lower())
    counter = Counter(words)

    # Build result
    result = {
        "total_words": len(words),
        "unique_words": len(counter),
        "top_words": [
            {"word": w, "count": c} for w, c in counter.most_common(top_n)
        ],
    }

    # Write output
    output_path = os.environ.get("SANDBOX_OUTPUT", "/sandbox/out/output.json")
    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with open(output_path, "w") as f:
        json.dump(result, f, indent=2)

    # Write report artifact
    files_dir = os.environ.get("SANDBOX_FILES_DIR", "/sandbox/out/files/")
    os.makedirs(files_dir, exist_ok=True)

    report_path = os.path.join(files_dir, "report.txt")
    with open(report_path, "w") as f:
        f.write("Word Count Report\n")
        f.write("=================\n\n")
        f.write(f"Total words: {len(words)}\n")
        f.write(f"Unique words: {len(counter)}\n\n")
        f.write("Top words:\n")
        for w, c in counter.most_common(top_n):
            f.write(f"  {w}: {c}\n")

    print(f"Counted {len(words)} words, {len(counter)} unique")


if __name__ == "__main__":
    main()
