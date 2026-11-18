#!/bin/sh
#
# os-pangolin state-change hook.
#
# The agent invokes this script with a single event argument after writing a
# new status/DNS snapshot. It must never interpolate untrusted data into a
# shell command; configctl validates its own action namespace.

exec /usr/local/sbin/configctl pangolin sync
