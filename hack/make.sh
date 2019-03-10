#!/bin/sh

# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# posix compliant
# verified by https://www.shellcheck.net

# Simple script to offer the ability to run the make directives in a Docker Container.
# In other words you don't need a Go env on your system to run Make (build, test etc)
# This script will just bind-mount the source directory into a container under the correct
# GOPATH and handle all of the Go ENV stuff for you.  All you need is Docker

# When in an interactive terminal add the -t flag so Docker inherits
# a pseudo TTY. Otherwords SIGINT does not work to kill the container
# when running this script interactively.
TERM_FLAGS="-i"
echo "${-}" | grep -q i && TERM_FLAGS="${TERM_FLAGS}t"

# shellcheck disable=2086
docker run --rm ${TERM_FLAGS} ${DOCKER_OPTS} \
  -v "$(pwd)":/build:z \
  -w /build \
  golang:1.11.5 make "${@}"
