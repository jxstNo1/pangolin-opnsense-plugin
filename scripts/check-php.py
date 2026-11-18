#!/usr/bin/env python3
"""Lint every PHP file under plugin/ without needing an OPNsense install."""
import pathlib
import subprocess
import sys

root = pathlib.Path(__file__).resolve().parent.parent
php = sys.argv[1] if len(sys.argv) > 1 else "php"
failed = 0
for path in (root / "plugin").rglob("*.php"):
    result = subprocess.run([php, "-l", str(path)], capture_output=True, text=True)
    if result.returncode != 0:
        failed += 1
        print(f"FAIL {path}")
        print(result.stdout, result.stderr)
print(f"php lint failures: {failed}")
raise SystemExit(1 if failed else 0)
