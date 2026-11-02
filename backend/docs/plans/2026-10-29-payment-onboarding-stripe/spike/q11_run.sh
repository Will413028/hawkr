#!/bin/sh
# usage: q11_run.sh  -> outputs/fixtures/*.json (each scrubbed)
set -e
cd "$(dirname "$0")"
set -a; . /Users/will/second-brain/secrets/personal/hawkr/hawkr-stripe-test.env; set +a
RAW=$(mktemp -d)
/opt/homebrew/bin/python3 -u q11_record.py "$RAW" 2>&1 | /opt/homebrew/bin/python3 scrub.py | tee outputs/q11_record.txt
mkdir -p outputs/fixtures
for f in "$RAW"/*.json; do /opt/homebrew/bin/python3 scrub.py < "$f" > "outputs/fixtures/$(basename "$f")"; done
rm -r -- "${RAW:?}"
