#! /bin/sh

# Copyright (c) 2025 The BFE Authors.
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

WORK_ROOT="$(cd "$(dirname "$0")" && pwd -P)"
VERSION=$(cat $WORK_ROOT/VERSION)
default_tag="latest"

if [ "$1" = "release" ]; then
  tag="v${VERSION}"
else
  tag=$default_tag
fi
echo "tag:$tag"

machine_arch=$(uname -m)
case $machine_arch in
  x86_64|i?86)
    arch="x86_64"
    ;;
  aarch64|armv8*)
    arch="aarch64"
    ;;
  armv7*|armv8l)
    echo "unsupport platform: ARM32"
    exit 1
    ;;
  *)
    echo "Unknown architecture: $ARCH"
    exit 1
    ;;
esac
echo "arch:$arch"

ftag="${arch}-${tag}"

# init git commit id
GIT_COMMIT=$(git rev-parse --short=8 HEAD) || true

FORMATTED_TS=$(date +"%y%m%d%H%M%S")
echo $FORMATTED_TS $VERSION $GIT_COMMIT

echo "${GIT_COMMIT}-${FORMATTED_TS}" >> IMAGEBUILD

docker build  --build-arg COMMIT_ID=${GIT_COMMIT} --build-arg VERSION=${VERSION} --build-arg FORMATTED_TS=${FORMATTED_TS} -t ghcr.io/bfenetworks/ai-gateway-controller:$ftag .
#docker push ghcr.io/bfenetworks/ai-gateway-controller:$ftag

