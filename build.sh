#!/bin/bash

set -euo pipefail

cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")"

REPO_PATH=github.com/kaloom/kubernetes-kactus-cni-plugin
EXEC_NAME=kactus
PROJECT_PATH=.gogradle/project_gopath

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

org_path=$(echo $REPO_PATH | cut -d/ -f 1-2)

mkdir -p $PROJECT_PATH

if [ ! -h ${PROJECT_PATH}/src/${REPO_PATH} ]; then
    mkdir -p ${PROJECT_PATH}/src/${org_path}
    ln -s ../../../../.. ${PROJECT_PATH}/src/${REPO_PATH}
fi

export GO15VENDOREXPERIMENT=1
export GOBIN=${PWD}/bin
export GOPATH=${PWD}/${PROJECT_PATH}
export GO111MODULE=off

echo "Building $EXEC_NAME"
go install -ldflags "${LDFLAGS}" "$@" ${REPO_PATH}/${EXEC_NAME}
