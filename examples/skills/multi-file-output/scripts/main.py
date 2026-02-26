"""Produces multiple file artifacts for testing."""
import json
import os

input_data = json.loads(os.environ.get("SANDBOX_INPUT", "{}"))
files_dir = os.environ.get("SANDBOX_FILES_DIR", "/sandbox/out/files/")
os.makedirs(files_dir, exist_ok=True)

file_count = input_data.get("file_count", 3)
files_written = []

# Write plain text files
for i in range(file_count):
    name = f"output_{i}.txt"
    path = os.path.join(files_dir, name)
    with open(path, "w") as f:
        f.write(f"File {i} content: hello from multi-file-output\n")
    files_written.append(name)

# Write a CSV
csv_path = os.path.join(files_dir, "data.csv")
with open(csv_path, "w") as f:
    f.write("id,value\n")
    for i in range(file_count):
        f.write(f"{i},{i*10}\n")
files_written.append("data.csv")

# Write into a subdirectory
subdir = os.path.join(files_dir, "nested")
os.makedirs(subdir, exist_ok=True)
nested_path = os.path.join(subdir, "deep.txt")
with open(nested_path, "w") as f:
    f.write("I am nested!\n")
files_written.append("nested/deep.txt")

# Write output
result = {
    "files_written": files_written,
    "total_files": len(files_written),
}

output_path = os.environ.get("SANDBOX_OUTPUT", "/sandbox/out/output.json")
os.makedirs(os.path.dirname(output_path), exist_ok=True)
with open(output_path, "w") as f:
    json.dump(result, f)

print(f"Wrote {len(files_written)} files")
