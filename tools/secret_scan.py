#!/usr/bin/env python3
"""Scan repository files for hardcoded secrets and sensitive credentials.

Exits with code 1 if any potential secrets are found, 0 otherwise.
Designed to run in CI or locally via `make secret-scan`.
"""
import os
import re
import sys

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

# Patterns that suggest a hardcoded secret.  Each entry is (label, regex).
SECRET_PATTERNS: list[tuple[str, re.Pattern]] = [
    ("AWS Access Key", re.compile(r"AKIA[0-9A-Z]{16}")),
    ("AWS Secret Key", re.compile(r"""(?:aws_secret_access_key|secret_access_key)\s*[:=]\s*['"]?[A-Za-z0-9/+=]{40}""", re.IGNORECASE)),
    ("Generic API Key assignment", re.compile(r"""(?:api_key|apikey|api_secret|secret_key)\s*[:=]\s*['"][A-Za-z0-9_\-]{20,}['"]""", re.IGNORECASE)),
    ("Bearer token", re.compile(r"""['"]Bearer\s+[A-Za-z0-9_\-\.]{20,}['"]""")),
    ("Private key block", re.compile(r"-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----")),
    ("GitHub token", re.compile(r"gh[pousr]_[A-Za-z0-9_]{36,}")),
    ("Generic secret in env", re.compile(r"""(?:PASSWORD|SECRET|TOKEN|CREDENTIAL)\s*=\s*['"][A-Za-z0-9_/+=\-\.]{16,}['"]""", re.IGNORECASE)),
    ("Slack webhook", re.compile(r"https://hooks\.slack\.com/services/T[A-Z0-9]+/B[A-Z0-9]+/[A-Za-z0-9]+")),
    ("JWT token", re.compile(r"eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}")),
]

# File extensions to scan.
SCAN_EXTENSIONS: set[str] = {
    ".go", ".ts", ".tsx", ".js", ".jsx", ".vue",
    ".py", ".sh", ".bash", ".zsh",
    ".yaml", ".yml", ".toml", ".json", ".env",
    ".cfg", ".conf", ".ini", ".properties",
    ".md", ".txt", ".dockerfile",
}

# Paths (relative to repo root) to skip entirely.
SKIP_DIRS: set[str] = {
    ".git", ".claude", "node_modules", "vendor", "dist", "build",
    "__pycache__", ".next", ".nuxt",
}

# Files to skip (basename).
SKIP_FILES: set[str] = {
    "pnpm-lock.yaml", "package-lock.json", "yarn.lock", "go.sum",
}

# Filename patterns to skip (checked with fnmatch).
SKIP_FILE_PATTERNS: list[str] = [
    "*_test.go",  # Go test files often contain fixture tokens
]

# Lines matching any of these are likely test fixtures or documentation.
FALSE_POSITIVE_HINTS: list[re.Pattern] = [
    re.compile(r"(?:example|placeholder|dummy|test|fake|mock|sample)", re.IGNORECASE),
    re.compile(r"(?:xxxx|your[_-]|changeme|replace[_-]me|TODO)", re.IGNORECASE),
    re.compile(r"secret_scan\.py"),  # don't flag ourselves
    re.compile(r"""(?:Field\w+\s*=|FieldName|const\s+\w+Field)"""),  # Go ORM field constants
    re.compile(r"""\$\{.*\}"""),  # template interpolation like ${token}
    re.compile(r"""(?:header|headers|localStorage|sessionStorage)\.""", re.IGNORECASE),  # runtime token access
    re.compile(r"""@[\w-]+=|v-on:|:[\w-]+="""  ),  # Vue event/prop bindings
    re.compile(r"""SettingKey\w+\s*="""),  # Go setting key constants (names, not values)
    re.compile(r"""GOCSPX-"""),  # Google OAuth public client secrets (expected in source)
]


# ---------------------------------------------------------------------------
# Scanner
# ---------------------------------------------------------------------------

def should_scan(filepath: str) -> bool:
    import fnmatch
    basename = os.path.basename(filepath)
    if basename in SKIP_FILES:
        return False
    if any(fnmatch.fnmatch(basename, pat) for pat in SKIP_FILE_PATTERNS):
        return False
    _, ext = os.path.splitext(basename)
    # Also scan Dockerfile (no extension match)
    if basename.lower().startswith("dockerfile"):
        return True
    return ext.lower() in SCAN_EXTENSIONS


def scan_file(filepath: str) -> list[tuple[int, str, str]]:
    """Return list of (line_number, label, matched_text) for a single file."""
    findings: list[tuple[int, str, str]] = []
    try:
        with open(filepath, "r", encoding="utf-8", errors="ignore") as fh:
            for lineno, line in enumerate(fh, start=1):
                stripped = line.strip()
                for label, pattern in SECRET_PATTERNS:
                    match = pattern.search(stripped)
                    if not match:
                        continue
                    # Filter out likely false positives
                    if any(fp.search(stripped) for fp in FALSE_POSITIVE_HINTS):
                        continue
                    findings.append((lineno, label, match.group()[:60]))
    except (OSError, UnicodeDecodeError):
        pass
    return findings


def main() -> int:
    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    total_findings: list[tuple[str, int, str, str]] = []

    for dirpath, dirnames, filenames in os.walk(repo_root):
        # Prune skipped directories in-place
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]

        for fname in filenames:
            fpath = os.path.join(dirpath, fname)
            if not should_scan(fpath):
                continue
            for lineno, label, snippet in scan_file(fpath):
                relpath = os.path.relpath(fpath, repo_root)
                total_findings.append((relpath, lineno, label, snippet))

    if not total_findings:
        print("✅ No secrets detected.")
        return 0

    print(f"❌ Found {len(total_findings)} potential secret(s):\n")
    for relpath, lineno, label, snippet in total_findings:
        # Mask the middle of the matched text
        if len(snippet) > 12:
            masked = snippet[:6] + "***" + snippet[-3:]
        else:
            masked = snippet[:3] + "***"
        print(f"  {relpath}:{lineno}  [{label}]  {masked}")

    print(f"\nPlease remove hardcoded secrets and use environment variables instead.")
    return 1


if __name__ == "__main__":
    sys.exit(main())
