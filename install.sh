#!/bin/bash

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
bin_dir="$HOME/.local/bin"
build_dir=$(mktemp -d)
trap 'rm -r "$build_dir"' EXIT

mkdir -p "$bin_dir"
make -s -C "$script_dir" build BUILD_DIR="$build_dir"

install -m 755 "$build_dir/ghtkn-touchid" "$bin_dir/ghtkn-touchid"
install -m 755 "$build_dir/ghtkn-touchid-reset" "$bin_dir/ghtkn-touchid-reset"

echo "Installed $bin_dir/ghtkn-touchid and $bin_dir/ghtkn-touchid-reset"
