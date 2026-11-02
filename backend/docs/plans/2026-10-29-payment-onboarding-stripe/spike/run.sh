#!/bin/sh
# usage: ./run.sh q01_countries.py  -> outputs/q01_countries.txt (scrubbed)
set -e
cd "$(dirname "$0")"
set -a; . /Users/will/second-brain/secrets/personal/hawkr/hawkr-stripe-test.env; set +a
n="${1%.py}"
/opt/homebrew/bin/python3 -u "$1" 2>&1 | /opt/homebrew/bin/python3 scrub.py | tee "outputs/$n.txt"
