# Step 9b (D55): does an update with contact_email land on an account that was already closed?
import uuid, json, base64, urllib.request, urllib.error, sp, q10_lib
q10_lib.VER = "2026-10-28.endive"
from q10_lib import v2
def create(key):
    h = {"Authorization": "Basic " + base64.b64encode((sp.KEY + ":").encode()).decode(), "Stripe-Version": "2026-10-28.endive", "Content-Type": "application/json", "Idempotency-Key": key}
    req = urllib.request.Request("https://api.stripe.com/v2/core/accounts", data=json.dumps({"metadata": {"store_id": "1009"}}).encode(), method="POST", headers=h)
    try:
        with urllib.request.urlopen(req) as r: return r.status, json.loads(r.read()), r.headers.get("Idempotent-Replayed")
    except urllib.error.HTTPError as e: return e.code, json.loads(e.read()), None
k = "spike-" + uuid.uuid4().hex
s, a, rep = create(k); aid = a["id"]; print("create shell", s, "replayed=", rep)
for cfg in ([], ["recipient"]):
    s, o = v2("POST", f"/v2/core/accounts/{aid}/close", {"applied_configurations": cfg})
    print("close shell", cfg, s, (o.get("error") or {}).get("code"), "closed=", o.get("closed"))
    if s == 200: break
s, b, rep = create(k); print("create shell again, same key", s, "same id=", b.get("id") == aid, "replayed=", rep, "closed=", b.get("closed"))
s, u = v2("POST", f"/v2/core/accounts/{aid}", {"contact_email": "spike@example.com", "identity": {"country": "us", "entity_type": "individual"},
    "configuration": {"recipient": {"capabilities": {"stripe_balance": {"stripe_transfers": {"requested": True}}}}},
    "defaults": {"responsibilities": {"fees_collector": "application", "losses_collector": "application"}}, "dashboard": "none"})
print("update closed account", s, (u.get("error") or {}).get("code"), (u.get("error") or {}).get("message"))
s, g = v2("GET", f"/v2/core/accounts/{aid}")
print("retrieve", s, {"closed": g.get("closed"), "contact_email_set": g.get("contact_email") is not None, "applied": g.get("applied_configurations")})
