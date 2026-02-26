"""Text summarization skill for Skillbox.

Uses extractive summarization: ranks sentences by importance
and selects the top N.
"""

import json
import math
import os
import re
import sys
from collections import Counter


def read_input():
    """Read input from SANDBOX_INPUT env var."""
    raw = os.environ.get("SANDBOX_INPUT", "")
    if raw:
        return json.loads(raw)
    input_path = "/sandbox/input.json"
    if os.path.exists(input_path):
        with open(input_path) as f:
            return json.load(f)
    return {}


def split_sentences(text):
    """Split text into sentences."""
    sentences = re.split(r"(?<=[.!?])\s+", text.strip())
    return [s.strip() for s in sentences if s.strip()]


def tokenize(text):
    """Simple word tokenization."""
    return re.findall(r"\b[a-z]+\b", text.lower())


def compute_tf(words):
    """Compute term frequency."""
    counter = Counter(words)
    total = len(words)
    return {word: count / total for word, count in counter.items()}


def compute_idf(sentences_words):
    """Compute inverse document frequency across sentences."""
    n = len(sentences_words)
    idf = {}
    all_words = set()
    for words in sentences_words:
        all_words.update(set(words))

    for word in all_words:
        containing = sum(1 for words in sentences_words if word in set(words))
        idf[word] = math.log(n / (1 + containing)) + 1

    return idf


def score_sentences(sentences):
    """Score sentences by TF-IDF importance."""
    sentences_words = [tokenize(s) for s in sentences]
    idf = compute_idf(sentences_words)

    scores = []
    for i, words in enumerate(sentences_words):
        if not words:
            scores.append(0)
            continue
        tf = compute_tf(words)
        score = sum(tf.get(w, 0) * idf.get(w, 0) for w in words)
        # Boost first sentences slightly (position bias)
        position_boost = 1.0 + (0.1 * (1.0 / (i + 1)))
        scores.append(score * position_boost)

    return scores


def summarize(text, max_sentences=3):
    """Extract top sentences as a summary."""
    sentences = split_sentences(text)

    if len(sentences) <= max_sentences:
        return " ".join(sentences), len(sentences)

    scores = score_sentences(sentences)

    # Get indices of top sentences, preserving original order
    ranked = sorted(range(len(scores)), key=lambda i: scores[i], reverse=True)
    top_indices = sorted(ranked[:max_sentences])

    summary_sentences = [sentences[i] for i in top_indices]
    return " ".join(summary_sentences), len(summary_sentences)


def main():
    input_data = read_input()

    text = input_data.get("text", "")
    max_sentences = input_data.get("max_sentences", 3)

    if not text:
        result = {
            "summary": "",
            "sentence_count": 0,
            "compression_ratio": 0,
            "error": "No text provided",
        }
    else:
        summary, sentence_count = summarize(text, max_sentences)
        compression_ratio = len(summary) / len(text) if text else 0

        result = {
            "summary": summary,
            "sentence_count": sentence_count,
            "compression_ratio": round(compression_ratio, 4),
            "original_length": len(text),
            "summary_length": len(summary),
        }

    output_path = os.environ.get("SANDBOX_OUTPUT", "/sandbox/out/output.json")
    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with open(output_path, "w") as f:
        json.dump(result, f, indent=2)

    print(f"Summary: {result.get('sentence_count', 0)} sentences, "
          f"compression ratio: {result.get('compression_ratio', 0)}")


if __name__ == "__main__":
    main()
