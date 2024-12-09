#!/usr/bin/env bash

set -eo pipefail

curl -f http://localhost | grep -q 'Hello Deployment'

tanjun shell -- deno --version
