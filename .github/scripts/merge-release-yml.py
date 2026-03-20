#!/usr/bin/env python3
"""Auto-merge release.yml conflicts: keep upstream DOCKERHUB_ENABLED + fork's Telegram/DockerHub steps"""
import re
import sys

try:
    with open('.github/workflows/release.yml', 'r') as f:
        content = f.read()
except FileNotFoundError:
    sys.exit(1)

# Find conflict markers
conflict_pattern = r'<<<<<<< HEAD\n(.*?)\n=======\n(.*?)\n>>>>>>> upstream/main'
match = re.search(conflict_pattern, content, re.DOTALL)

if not match:
    sys.exit(0)  # No conflict

ours = match.group(1)
theirs = match.group(2)

# Extract upstream's DOCKERHUB_USERNAME line with DOCKERHUB_ENABLED logic
upstream_line = re.search(r'DOCKERHUB_USERNAME:.*DOCKERHUB_ENABLED.*', theirs)

# Extract fork's custom steps (DockerHub description + Telegram notification)
fork_steps = re.search(
    r'(# Update DockerHub description.*?# Send Telegram Notification.*?sendMessage.*?\n\s+\}\'\)\")',
    theirs,
    re.DOTALL
)

if upstream_line and fork_steps:
    # Merge: upstream logic + fork steps
    merged = f"{upstream_line.group(0)}\n\n{fork_steps.group(1)}"
    result = content.replace(match.group(0), merged)

    with open('.github/workflows/release.yml', 'w') as f:
        f.write(result)
    sys.exit(0)
else:
    sys.exit(1)  # Cannot auto-merge
