#!/usr/bin/env python3
"""Validate OPNsense model/form/menu/ACL XML files with the stdlib parser."""
import pathlib
import sys
import xml.etree.ElementTree as ET

root = pathlib.Path(__file__).resolve().parent.parent
failed = 0
for path in (root / "plugin").rglob("*.xml"):
    try:
        ET.parse(path)
    except ET.ParseError as exc:
        failed += 1
        print(f"FAIL {path}: {exc}")
print(f"xml parse failures: {failed}")
raise SystemExit(1 if failed else 0)
