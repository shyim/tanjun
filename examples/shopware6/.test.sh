#!/usr/bin/env bash

set -eox pipefail

curl -f http://localhost/admin | grep -q 'Shopware Administration'

tanjun shell -- bin/console plugin:refresh
