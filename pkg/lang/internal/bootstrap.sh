#!/usr/bin/env bash

set -xeuo pipefail

# Arguments are the package name and base name of the source file containing
# the `//go generate` directive.
declare -r pkg=${1}
declare -r src=${2}

# First, cd to the root of the repo.
declare _top
_top=$( cd "${0%/*}"; git rev-parse --show-toplevel )
pushd "${_top}" &>/dev/null

# Use the "internal" package relative to our `//go generate` directive.
declare -a _dir
mapfile -n 1 -t _dir < <( go list -json ./... | jq -rs "
  .[] | select(.Dir | endswith(\"/${pkg}\")) | select(.GoFiles[]? == \"${src}\") | .Dir
" )
pushd "${_dir[0]}/internal" &>/dev/null

go tool gogll -a -bs -v "grammar.md"

mkdir -p ".gogll"
mv ./*.txt ".gogll/"
popd &>/dev/null

unset -v _top _dir
