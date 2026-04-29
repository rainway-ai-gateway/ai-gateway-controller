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

#---step 1
FROM golang:1.24-alpine AS build
RUN apk update && apk add git tzdata

ARG COMMIT_ID
ARG VERSION
ARG FORMATTED_TS

ENV SC_COMMIT_ID=${COMMIT_ID}
ENV SC_VERSION=${VERSION}
ENV SC_FORMATTED_TS=${FORMATTED_TS}

WORKDIR /ai-gateway-controller

RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime 
RUN echo "Asia/Shanghai" > /etc/timezone

# Download go modules
COPY go.mod .
COPY go.sum .
RUN GO111MODULE=on GOPROXY=https://goproxy.cn,direct go mod download
#RUN GO111MODULE=on go mod download

COPY . .
RUN chmod +x build/build.sh
RUN sh build/build.sh docker


#---step 2
FROM alpine:3.23 AS run

ARG COMMIT_ID
ARG VERSION
ARG FORMATTED_TS

ENV SC_COMMIT_ID=${COMMIT_ID}
ENV SC_VERSION=${VERSION}
ENV SC_FORMATTED_TS=${FORMATTED_TS}

LABEL org.opencontainers.image.revision=${COMMIT_ID} \
      org.opencontainers.image.version=${VERSION} \
      org.opencontainers.image.created=${FORMATTED_TS} \
      org.opencontainers.image.revcreated=${COMMIT_ID}-${FORMATTED_TS} \
      security.recommendations="non-root user, read-only rootfs, no-new-privileges, no privileged mode"

#RUN yum install -y shadow
#RUN groupadd -r bfegroup -g 1000 && useradd -r -g bfegroup -u 1000 bfeuser
RUN echo "bfegroup:x:1000:" >> /etc/group
RUN echo "work:x:1000:1000::/home/work:/bin/sh" >> /etc/passwd
RUN mkdir -p /home/work

WORKDIR /
COPY --from=build /ai-gateway-controller/output/* /

EXPOSE 9080 9091

USER work:bfegroup

ENTRYPOINT ["/ai-gateway-controller"]


