#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$(readlink -f "../${BASH_SOURCE[0]}")")"

[ -x bin/kactus ] || (echo "please build the kactus first by running ./build.sh"; exit 1)

. gradle.properties

docker build . -t kaloom/kactus:$version
