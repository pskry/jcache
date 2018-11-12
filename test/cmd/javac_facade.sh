#!/bin/sh

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"

go run ${SCRIPT_DIR}/../../cmd/jcache.go /usr/bin/javac $*
