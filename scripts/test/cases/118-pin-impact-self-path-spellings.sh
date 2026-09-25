#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 118 (go-kure/kure#888 review): `$0` under other spellings is refused — a
# positional slice `${@:0:1}`, `${*:0:1}`, `${@: -2}` (any offset can reach
# $0), BASH_ARGV — and a message naming `$0` is exempt only when it goes to
# stderr: `echo "$0" >&3`, or with no redirect at all, can be read back.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "scripts/tool.py"

while IFS= read -r body <&3; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\n' "$body")"
    pi_run
    pi_expect 1 "unrecognized script-directory expression in scripts/check-a.sh"
done 3<<'LINES'
x="${@:0:1}"
x="${*:0:1}"
x="${@: -2:1}"
x="${BASH_ARGV[0]}"
echo "$0" >&3
echo "$0" >"$RUNNER_TEMP/self"
self() { echo "usage: $0"; }
{ echo "usage: $0" 2>&1; }
LINES

# A usage message to stderr, and a default for "$@", name no path.
pi_raw "$PI_NEW" scripts/check-a.sh "$(cat <<'BODY'
#!/bin/bash
[[ -n "${MODE:-}" ]] || { echo "usage: $0 --full-tree | --diff BASE" >&2; exit 2; }
[[ $# -gt 0 ]] || echo "usage: $0 ARG" 1>&2
set -- "${@:-default}"
BODY
)"
pi_run
pi_expect 0 "pin refresh is inert"
