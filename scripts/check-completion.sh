#!/usr/bin/env bash
set -euo pipefail

binary=$1
completion_dir=$(mktemp -d "${TMPDIR:-/tmp}/dmut-completion.XXXXXX")
trap 'rm -rf -- "$completion_dir"' EXIT

"$binary" --completion bash > "$completion_dir/dmut.bash"
"$binary" --completion zsh > "$completion_dir/_dmut"
"$binary" --completion fish > "$completion_dir/dmut.fish"

bash -n "$completion_dir/dmut.bash"
source "$completion_dir/dmut.bash"
COMP_WORDS=(dmut --pre)
COMP_CWORD=1
_dmut_complete
[[ " ${COMPREPLY[*]} " == *" --preview "* ]]
[[ " ${COMPREPLY[*]} " == *" --preview-limit "* ]]

zsh -n "$completion_dir/_dmut"
fish --no-config -n "$completion_dir/dmut.fish"
fish_matches=$(fish --no-config -c 'source $argv[1]; complete -C "dmut --pre"' "$completion_dir/dmut.fish")
[[ "$fish_matches" == *"--preview"* ]]
[[ "$fish_matches" == *"--preview-limit"* ]]
printf '%s\n' 'Bash, Zsh and Fish completion checks passed.'
