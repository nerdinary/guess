#!/bin/sh
q="$*"
[ -z "$q" ] && q="$(xclip -o)"
exec $(dirname $0)/guess "$q"
