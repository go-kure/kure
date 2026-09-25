#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 110 (go-kure/kure#888): the script's own path, `$0`, is refused anywhere
# but in a message, a `sed -n '<lines>p' "$0"` read of the script itself, or
# an awk program's record (`f($0`, `,$0`, `= $0`, `$0 ~`); so is BASH_ARGV0.
# Carried through a variable (`x=$0`, `a=($0)`, `printf -v`, `read <<<`, a
# function argument) it could name a sibling no scan follows, and read as
# inert.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "scripts/tool.py"

while IFS= read -r body <&3; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\n' "$body")"
    pi_run
    pi_expect 1 "unrecognized script-directory expression in scripts/check-a.sh"
done 3<<'LINES'
x=$0
helper="$0"
printf -v x '%s' "$0"
read -r x <<<"$0"
run() { python3 "${1%/*}/tool.py"; }; run "$0"
x=$BASH_ARGV0
a=($0)
LINES

# The x=$0 of the issue, with the call two lines on.
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nx=$0\ntool="${x%%/*}/tool.py"\npython3 "$tool"\n')"
pi_run
pi_expect 1 "  x=\$0"

# A message, a read of the script's own text and awk records are no path.
pi_raw "$PI_NEW" scripts/check-a.sh "$(cat <<'BODY'
#!/bin/bash
-h|--help) sed -n '2,29p' "$0"; exit 0 ;;
[[ -n "$MODE" ]] || { echo "usage: $0 --full-tree | --diff BASE" >&2; exit 2; }
awk '
  { ind = indent($0); ref = $0 }
  /^@@ / { if (match($0, /x/)) n = substr($0, 2) }
  { if ($0 !~ /y/) next }
' file
BODY
)"
pi_run
pi_expect 0 "pin refresh is inert"
