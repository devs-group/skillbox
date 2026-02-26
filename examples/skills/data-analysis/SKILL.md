---
name: data-analysis
version: "1.0.0"
description: Analyze CSV data and produce summary statistics with a chart
lang: python
image: python:3.12-slim
timeout: 60s
resources:
  memory: 256Mi
  cpu: "0.5"
---

# Data Analysis Skill

Analyze CSV or JSON data and produce summary statistics.

## Input

Provide data as a JSON object with a `data` field containing an array of records,
or a `csv` field containing raw CSV text.

## Output

Writes a JSON summary to output.json with descriptive statistics per column.
If the data has numeric columns, writes a `summary_chart.png` bar chart to the
files output directory.
