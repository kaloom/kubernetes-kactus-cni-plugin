#!/bin/bash

set -euo pipefail

cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")"

GOGRADLE_PROJECT_PATH=.gogradle/project_gopath

# Add branch/commit/date into binary
set +e
git describe --tags --abbrev=0 > /dev/null 2>&1
if [ "$?" != "0" ]; then
    BRANCH="master"
else
    BRANCH=$(git describe --tags --abbrev=0)
fi

set -e
DATE=$(date --utc "+%F_%H:%m:%S_+0000")
COMMIT=$(git rev-parse --verify --short HEAD)
LDFLAGS="-X main.branch=${BRANCH:-master} -X main.commit=${COMMIT} -X main.date=${DATE}"

. gradle.properties

repo_path=$packageRepo
exec_name=$execName
org_path=$(echo $repo_path | cut -d/ -f 1-2)

mkdir -p $GOGRADLE_PROJECT_PATH

if [ ! -h ${GOGRADLE_PROJECT_PATH}/src/${repo_path} ]; then
    mkdir -p ${GOGRADLE_PROJECT_PATH}/src/${org_path}
    ln -s ../../../../.. ${GOGRADLE_PROJECT_PATH}/src/${repo_path}
fi

export GO15VENDOREXPERIMENT=1
export GOBIN=${PWD}/bin
export GOPATH=${PWD}/${GOGRADLE_PROJECT_PATH}
export GO111MODULE=off

echo "Building $exec_name"
go install -ldflags "${LDFLAGS}" "$@" ${repo_path}/${exec_name}
