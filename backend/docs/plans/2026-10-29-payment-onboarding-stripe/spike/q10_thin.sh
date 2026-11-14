#!/bin/sh
# Q5: stripe listen -> local receiver; output is scrubbed by the caller. Secret never printed.
cd "$(dirname "$0")"
set -a; . /Users/will/second-brain/secrets/personal/hawkr/hawkr-stripe-test.env; set +a
SCR=/private/tmp/claude-501/-Users-will-second-brain--projects-personal-hawkr/7ead3f72-2141-4176-8a1e-b87d4c0829f6/scratchpad
LOG=$SCR/deliveries.jsonl; : > "$LOG"
WEBHOOK_SIGNING_SECRET=$(/opt/homebrew/bin/stripe listen --print-secret --api-key "$STRIPE_SECRET_KEY")
export WEBHOOK_SIGNING_SECRET
/opt/homebrew/bin/python3 webhook_receiver.py "$LOG" 4242 &
RPID=$!
/opt/homebrew/bin/stripe listen --api-key "$STRIPE_SECRET_KEY" --all-thin --forward-to http://127.0.0.1:4242/hook --forward-connect-to http://127.0.0.1:4242/hook > "$SCR/listen.log" 2>&1 &
LPID=$!
sleep 8
/opt/homebrew/bin/python3 -u q10_thin_drive.py "$LOG" "$SCR/listen.log"
kill $LPID $RPID 2>/dev/null
