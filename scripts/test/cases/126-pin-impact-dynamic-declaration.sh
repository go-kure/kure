#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 126 (go-kure/kure#888 review): `declare -g "$n+=/lib"`, with n built as
# SCRIPT_DIR at run time, repoints SCRIPT_DIR under a name the scan never
# sees: the source below it runs scripts/lib/y.sh while the scan followed
# scripts/y.sh, and read as inert. A declare/typeset/local/export/readonly
# whose variable name holds a `$` or a backtick is refused.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/y.sh "echo y"
pi_compare ahead "scripts/lib/y.sh"
def='SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"'

while IFS= read -r decl <&3; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\nn=SCRIPT_; n+=DIR\n%s\nsource "$SCRIPT_DIR/y.sh"\n' "$def" "$decl")"
    pi_run
    pi_expect 1 "a declaration whose variable name is built at run time in scripts/check-a.sh"
done 3<<'DECLS'
declare -g "$n+=/lib"
typeset -g ${n}+=/lib
f() { local x; export "$n"="$x/lib"; }
readonly "$n"
x=1 declare -g "$n=/tmp"
DECLS

# Names written out, a value with `$` in it, a compound array value and the
# word in a message stay as they were.
pi_compare ahead "scripts/y.sh"
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\nlocal pat="$1" a b\nexport PATH="$HOME/bin:$PATH"\ndeclare -A m=([$k]=v)\necho "declare $x"\nsource "$SCRIPT_DIR/y.sh"\n' "$def")"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  scripts/y.sh"
