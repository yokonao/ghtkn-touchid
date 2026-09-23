#!/bin/bash

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
bin_dir="$HOME/.local/bin"
build_dir=$(mktemp -d)
trap 'rm -r "$build_dir"' EXIT

mkdir -p "$bin_dir"
make -s -C "$script_dir" build BUILD_DIR="$build_dir"

install -m 755 "$build_dir/ghtkn-touchid" "$bin_dir/ghtkn-touchid"

echo "Installed $bin_dir/ghtkn-touchid"
