#!/usr/bin/env bash
# scripts/demo.sh
#
# Scripted demo for asciinema recording. Run with:
#   asciinema rec demo/demo.cast -c "bash scripts/demo.sh"
#
# Requires: tdx binary in PATH, authenticated session.
# The script pauses between commands for readable pacing.

set -euo pipefail

pause() { sleep 1.5; }

echo "# tdx — TeamDynamix time entries from the terminal"
echo ""
pause

echo '$ tdx version'
tdx version
pause

echo ""
echo '$ tdx auth status'
tdx auth status
pause

echo ""
echo '$ tdx time entry list'
tdx time entry list
pause

echo ""
echo '$ tdx time week show'
tdx time week show
pause

echo ""
echo '$ tdx time template derive demo-week --from-week $(date +%Y-%m-%d)'
tdx time template derive demo-week --from-week "$(date +%Y-%m-%d)" 2>/dev/null || echo "(template already exists or no entries this week)"
pause

echo ""
echo '$ tdx time template show demo-week'
tdx time template show demo-week 2>/dev/null || echo "(no template to show)"
pause

echo ""
echo "# Clean up"
echo '$ tdx time template delete demo-week'
tdx time template delete demo-week 2>/dev/null || true
pause

echo ""
echo "# Done! See https://github.com/iainmoffat/tdx for more."
