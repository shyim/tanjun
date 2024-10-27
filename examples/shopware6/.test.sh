#!/usr/bin/env bash

set -eox pipefail

curl -f http://localhost | grep -q 'window.router'

tanjun shell -- bin/console plugin:refresh
