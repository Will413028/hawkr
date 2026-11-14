# Step 9b (D55): replaying the metadata-only create with the same idempotency key: open account, updated account, closed account.
import time, uuid, json, base64, urllib.request, urllib.error, sp, q10_lib
q10_lib.VER = "2026-10-28.endive"
from q10_lib import v2
def create(key):
    h = {"Authorization": "Basic " + base64.b64encode((sp.KEY + ":").encode()).decode(), "Stripe-Version": "2026-10-28.endive", "Content-Type": "application/json", "Idempotency-Key": key}
    req = urllib.request.Request("https://api.stripe.com/v2/core/accounts", data=json.dumps({"metadata": {"store_id": "1009"}}).encode(), method="POST", headers=h)
    try:
        with urllib.request.urlopen(req) as r: return r.status, json.loads(r.read()), r.headers.get("Idempotent-Replayed")
    except urllib.error.HTTPError as e: return e.code, json.loads(e.read()), None
def show(label, s, o, rep, aid=None):
    e = o.get("error") or {}
    print(label, s, "same id=" + str(o.get("id") == aid) if s == 200 else {"code": e.get("code"), "message": e.get("message")}, "replayed=", rep)
k = "spike-" + uuid.uuid4().hex
s, a, rep = create(k); aid = a["id"]; print("create shell", s, "replayed=", rep)
s, o, rep = create(k); show("replay immediately (open, bare)", s, o, rep, aid)
time.sleep(5)
s, o, rep = create(k); show("replay after 5s (open, bare)", s, o, rep, aid)
s, u = v2("POST", f"/v2/core/accounts/{aid}", {"contact_email": "spike@example.com", "identity": {"country": "us", "entity_type": "individual"},
    "configuration": {"recipient": {"capabilities": {"stripe_balance": {"stripe_transfers": {"requested": True}}}}},
    "defaults": {"responsibilities": {"fees_collector": "application", "losses_collector": "application"}}, "dashboard": "none"})
print("update", s)
s, o, rep = create(k); show("replay after update (open, configured)", s, o, rep, aid)
print("close", v2("POST", f"/v2/core/accounts/{aid}/close", {"applied_configurations": ["recipient"]})[0])
s, o, rep = create(k); show("replay after close", s, o, rep, aid)
time.sleep(5)
s, o, rep = create(k); show("replay after close +5s", s, o, rep, aid)
