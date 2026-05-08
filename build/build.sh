#! /bin/sh

# Copyright(c) 2026 Beijing Yingfei Networks Technology Co.Ltd. 
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#http: //www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.

# Copyright (c) 2026 The BFE Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -e
set -x

WORK_ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
echo "WORK_ROOT: $WORK_ROOT"

BUILD_MODE="${1:-}"

function go_test() {
    cd ${WORK_ROOT}
    export CGO_ENABLED=1
	go test -race -gcflags=-l -cover ./...
	#go test -gcflags=-l -cover ./...
    if [ $? -ne 0 ];
    then
        echo "go test failed"
        exit 1
    fi
    echo "OK for go test"
}
if [ "$BUILD_MODE" != "docker" ]; then
    go_test
fi

function go_vet() {
    cd ${WORK_ROOT}
    # run go vet for all subdirectory
    # Note: vet uses heuristics that do not guarantee all reports are
    # genuine problems, but it can find errors not caught by the compilers.
    go vet ./...
    if [ $? -ne 0 ];
    then
        echo "go vet failed"
        exit 1
    fi

    echo "OK for go vet"
}
if [ "$BUILD_MODE" != "docker" ]; then
    go_vet
fi

cd "${WORK_ROOT}"
# init version
VERSION=$(cat $WORK_ROOT/VERSION)
# init git commit id
GIT_COMMIT=$(git rev-parse HEAD) || true

go build -ldflags "-X main.version=${VERSION} -X main.commit=${GIT_COMMIT}" \
	-o $WORK_ROOT/output/ai-gateway-controller $WORK_ROOT/cmd/ai-gateway-controller

# test
# go test ./...

# set permission for docker
chmod a+x $WORK_ROOT/output/*
echo "${GIT_COMMIT}" > $WORK_ROOT/output/ai-gateway-controller.commit
