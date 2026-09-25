#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 127 (go-kure/kure#888 review): `$_` is the last argument of the previous
# command, so right after an exempted `echo "usage: $0" >&2` or `sed -n '1p'
# "$0"` it holds the script's own path, and `${self%/*}/tool.py` reached a
# file the scan never saw. `$_` and `${_...}` are refused in a walked script.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "scripts/tool.py"

while IFS= read -r body <&3; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\n' "$body")"
    pi_run
    pi_expect 1 "unrecognized script-directory expression in scripts/check-a.sh"
done 3<<'BODIES'
[[ $# -gt 0 ]] || echo "usage: $0" >&2; self=$_; python3 "${self%/*}/tool.py"
sed -n '1p' "$0" >/dev/null; python3 "$(dirname "${_}")/tool.py"
echo "usage: $0" >&2; python3 "${_%/*}/tool.py"
BODIES

# `$__x` and `$_x` are other variables.
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n_x=1; __y=2; echo "$_x $__y"\n')"
pi_run
pi_expect 0 "pin refresh is inert"
