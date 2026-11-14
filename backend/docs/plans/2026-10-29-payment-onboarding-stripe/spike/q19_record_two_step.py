# Step 9b (D55): record the two-step creation responses for the adapter's contract tests (Stripe-Version 2026-10-28.endive).
# Writes one {status, body} JSON per call to argv[1]; the caller scrubs each file into outputs/fixtures/.
import base64, json, sys, urllib.error, urllib.request, uuid
import sp

VER = "2026-10-28.endive"
OUT = sys.argv[1]
INC = ["configuration.recipient", "requirements", "identity", "defaults"]

def v2(path, body, idem=None):
    h = {"Authorization": "Basic " + base64.b64encode((sp.KEY + ":").encode()).decode(), "Stripe-Version": VER, "Content-Type": "application/json"}
    if idem: h["Idempotency-Key"] = idem
    req = urllib.request.Request("https://api.stripe.com" + path, data=json.dumps(body).encode(), method="POST", headers=h)
    try:
        with urllib.request.urlopen(req) as r: s, t = r.status, r.read().decode()
    except urllib.error.HTTPError as e: s, t = e.code, e.read().decode()
    sp._check(json.loads(t)); return s, t

def save(name, s, t):
    with open(f"{OUT}/{name}.json", "w") as f:
        f.write(json.dumps({"status": s, "body": json.loads(t)}, indent=2, ensure_ascii=False))
    print(name, s)

for mode, dash in [("platform", "none"), ("provider", "express")]:
    s, t = v2("/v2/core/accounts", {"metadata": {"store_id": "1009", "hawkr_namespace": "spike"}}, "spike-" + uuid.uuid4().hex); save(f"create_shell_{mode}", s, t)
    aid = json.loads(t)["id"]
    s, t = v2(f"/v2/core/accounts/{aid}", {"contact_email": "spike@example.com", "identity": {"country": "us", "entity_type": "individual"},
        "configuration": {"recipient": {"capabilities": {"stripe_balance": {"stripe_transfers": {"requested": True}}}}},
        "defaults": {"responsibilities": {"fees_collector": "application", "losses_collector": "application"}}, "dashboard": dash, "include": INC})
    save(f"update_{mode}", s, t)
    print("close", v2(f"/v2/core/accounts/{aid}/close", {"applied_configurations": ["recipient"]})[0])
