#!/bin/sh

set -eux

base_dir=$(dirname "$(realpath "$0")")
workdir=$(mktemp -d)
forest_cmd="$workdir/forest"

env --chdir="$base_dir" go build -o "$forest_cmd"

cd "$workdir"
identity=$("$forest_cmd" create identity)
community=$("$forest_cmd" create community --as "$identity")
reply1=$("$forest_cmd" create reply --as "$identity" --to "$community" --content test1)
reply2=$("$forest_cmd" create reply --as "$identity" --to "$reply1" --content test2)
reply3=$("$forest_cmd" create reply --as "$identity" --to "$reply1" --content "this node has metadata" --metadata '{"meta/1": "some-value" }')

"$forest_cmd" show "$identity"
"$forest_cmd" show "$community"
"$forest_cmd" show "$reply1"
"$forest_cmd" show "$reply2"
"$forest_cmd" show "$reply3"
