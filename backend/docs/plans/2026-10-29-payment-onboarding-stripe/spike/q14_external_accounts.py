# Step 6: record the v1 external bank account list of the step 5b sandbox account (Stripe-Version 2026-10-28.endive).
# Prints the {status, body} JSON to stdout; the caller pipes it through scrub.py into outputs/fixtures/.
import base64, json, sys, urllib.error, urllib.request
import sp

AID = sys.argv[1]
req = urllib.request.Request(
    f"https://api.stripe.com/v1/accounts/{AID}/external_accounts?object=bank_account&limit=100",
    headers={"Authorization": "Basic " + base64.b64encode((sp.KEY + ":").encode()).decode(), "Stripe-Version": "2026-10-28.endive"})
try:
    with urllib.request.urlopen(req) as r:
        s, t = r.status, r.read().decode()
except urllib.error.HTTPError as e:
    s, t = e.code, e.read().decode()
body = json.loads(t)
sp._check(body)
print(json.dumps({"status": s, "body": body}, indent=2, ensure_ascii=False))
