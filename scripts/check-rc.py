#!/usr/bin/env python3
"""Regression check for the rc.d daemon/agent command split.

The service PID belongs to /usr/sbin/daemon, so ``procname`` must stay the
daemon path for rc.subr liveness checks. The child command in ``command_args``
must be the agent binary, not ``${procname}``. Reusing ``${procname}`` there
makes daemon try to execute "/usr/sbin/daemon run ..." and the agent never
starts.
"""
import pathlib
import re
import sys

root = pathlib.Path(__file__).resolve().parent.parent
agent = "/usr/local/sbin/pangolin-agent"
failed = 0

for name in ("pangolin_client", "pangolin_site"):
    path = root / "plugin" / "src" / "etc" / "rc.d" / name
    text = path.read_text(encoding="utf-8")
    checks = {
        "agent variable": f'agent="{agent}"' in text,
        "daemon command": 'command="/usr/sbin/daemon"' in text,
        "daemon procname": 'procname="/usr/sbin/daemon"' in text,
        "command_args uses agent": bool(
            re.search(r"command_args=.*\$\{agent\} run --config", text)
        ),
        "precmd checks agent": 'if [ ! -x "${agent}" ]; then' in text,
        "no procname as child command": "${procname} run --config" not in text,
    }
    for label, ok in checks.items():
        if not ok:
            failed += 1
            print(f"FAIL {path}: {label}")

print(f"rc.d script failures: {failed}")
raise SystemExit(1 if failed else 0)
