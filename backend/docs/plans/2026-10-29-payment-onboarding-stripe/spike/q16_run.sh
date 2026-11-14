#!/bin/sh
# usage: q16_run.sh -> outputs/q16_ga_version_close.txt (scrubbed); Stripe sandbox key
set -e
cd "$(dirname "$0")"
set -a; . /Users/will/second-brain/secrets/personal/hawkr/hawkr-stripe-sandbox.env; set +a
/opt/homebrew/bin/python3 -u q16_ga_version_close.py 2>&1 | /opt/homebrew/bin/python3 scrub.py | tee outputs/q16_ga_version_close.txt
