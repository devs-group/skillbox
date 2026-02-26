"""PDF text extraction skill for Skillbox.

Extracts text from base64-encoded PDF content using a lightweight
pure-Python approach.
"""

import base64
import json
import os
import re
import sys


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


def extract_text_simple(pdf_bytes):
    """Simple PDF text extraction using regex.

    This is a lightweight approach that works for text-based PDFs.
    For production use, consider adding PyPDF2 to requirements.txt.
    """
    content = pdf_bytes.decode("latin-1", errors="replace")

    # Find text between BT and ET markers (PDF text objects)
    pages = []
    page_texts = re.split(r"/Type\s*/Page[^s]", content)

    for i, page_content in enumerate(page_texts[1:], 1):
        text_objects = re.findall(r"BT\s*(.*?)\s*ET", page_content, re.DOTALL)
        page_text = ""
        for obj in text_objects:
            # Extract text from Tj and TJ operators
            strings = re.findall(r"\((.*?)\)", obj)
            page_text += " ".join(strings) + "\n"

        if page_text.strip():
            pages.append({
                "page_number": i,
                "text": page_text.strip(),
            })

    return pages


def main():
    input_data = read_input()

    if "pdf_base64" in input_data:
        pdf_bytes = base64.b64decode(input_data["pdf_base64"])
        pages = extract_text_simple(pdf_bytes)

        # Also save raw PDF as artifact
        files_dir = os.environ.get("SANDBOX_FILES_DIR", "/sandbox/out/files/")
        os.makedirs(files_dir, exist_ok=True)

        extracted_path = os.path.join(files_dir, "extracted_text.txt")
        with open(extracted_path, "w") as f:
            for page in pages:
                f.write(f"--- Page {page['page_number']} ---\n")
                f.write(page["text"])
                f.write("\n\n")

    elif "text" in input_data:
        # Fallback: process plain text
        text = input_data["text"]
        pages = [{"page_number": 1, "text": text}]
    else:
        pages = []

    total_chars = sum(len(p["text"]) for p in pages)

    result = {
        "pages": pages,
        "total_pages": len(pages),
        "total_characters": total_chars,
    }

    output_path = os.environ.get("SANDBOX_OUTPUT", "/sandbox/out/output.json")
    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with open(output_path, "w") as f:
        json.dump(result, f, indent=2)

    print(f"Extracted {len(pages)} pages, {total_chars} characters")


if __name__ == "__main__":
    main()
