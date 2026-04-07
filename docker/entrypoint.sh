#!/bin/sh
set -eu

if [ "${GH_TOKEN:-}" = "" ] && [ "${GITHUB_TOKEN:-}" = "" ]; then
    echo "missing required environment variable: GITHUB_TOKEN (or GH_TOKEN)" >&2
    exit 21
fi

if [ "${GH_TOKEN:-}" = "" ]; then
    export GH_TOKEN="${GITHUB_TOKEN}"
fi

git config --global user.name "${GIT_USER_NAME:-moltenhub-bot}"
git config --global user.email "${GIT_USER_EMAIL:-moltenhub-bot@users.noreply.github.com}"

if ! git config --global --get-all url."https://github.com/".insteadOf 2>/dev/null | grep -Fxq "git@github.com:"; then
    git config --global --add url."https://github.com/".insteadOf "git@github.com:"
fi
if ! git config --global --get-all url."https://github.com/".insteadOf 2>/dev/null | grep -Fxq "ssh://git@github.com/"; then
    git config --global --add url."https://github.com/".insteadOf "ssh://git@github.com/"
fi

gh auth status >/dev/null
gh auth setup-git >/dev/null

# Keep /usr/local/bin available even when runtime env overrides PATH.
export PATH="/usr/local/bin:${PATH:-/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin}"

if [ "$#" -eq 0 ]; then
    set -- /usr/local/bin/harness
fi

if [ "${1#*/}" = "$1" ] && [ -x "/usr/local/bin/$1" ]; then
    cmd="/usr/local/bin/$1"
    shift
    set -- "$cmd" "$@"
fi

exec "$@"
