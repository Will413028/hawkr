# Step 9b: can a v2 account be created with a body that has no mutable owner data (no contact_email, no recipient
# configuration) and get the email and the recipient configuration in a later update?
import uuid, sp, q10_lib
q10_lib.VER = "2026-10-28.endive"
from q10_lib import v2

def err(o):
    e = o.get("error") or {}
    return {k: e.get(k) for k in ("code", "message") if e.get(k)}

def caps(a):
    sb = (((a.get("configuration") or {}).get("recipient") or {}).get("capabilities") or {}).get("stripe_balance") or {}
    return {k: (v or {}).get("status") for k, v in sb.items()}

RECIPIENT = {"recipient": {"capabilities": {"stripe_balance": {"stripe_transfers": {"requested": True}}}}}
RESP = {"responsibilities": {"fees_collector": "application", "losses_collector": "application"}}
IDENT = {"country": "us", "entity_type": "individual"}
for dashboard in ("none", "express"):
    for label, body in [
        ("bare: identity+defaults+dashboard+metadata", {"identity": IDENT, "defaults": RESP, "dashboard": dashboard, "metadata": {"store_id": "1009"}}),
        ("bare without defaults", {"identity": IDENT, "dashboard": dashboard, "metadata": {"store_id": "1009"}}),
        ("metadata only", {"metadata": {"store_id": "1009"}}),
    ]:
        s, a = v2("POST", "/v2/core/accounts", dict(body, include=["configuration.recipient", "defaults", "identity"]))
        print(f"[{dashboard}] create {label}:", s, {"dashboard": a.get("dashboard"), "metadata": a.get("metadata"), "applied": a.get("applied_configurations")} if s == 200 else err(a))
        if s != 200: continue
        aid = a["id"]
        s, o = v2("POST", "/v2/core/account_links", {"account": aid, "use_case": {"type": "account_onboarding", "account_onboarding": {"return_url": "https://accessible.stripe.com/r", "refresh_url": "https://accessible.stripe.com/f"}}})
        print("   link before update:", s, "ok" if s == 200 else err(o))
        upd = {"contact_email": "spike@example.com", "configuration": RECIPIENT, "include": ["configuration.recipient", "defaults"]}
        if "defaults" not in body: upd["defaults"] = RESP
        if "identity" not in body: upd["identity"] = IDENT
        if "dashboard" not in body: upd["dashboard"] = dashboard
        s, u = v2("POST", f"/v2/core/accounts/{aid}", upd)
        print("   update email+recipient:", s, {"caps": caps(u), "applied": u.get("applied_configurations"), "dashboard": u.get("dashboard"), "resp": (u.get("defaults") or {}).get("responsibilities")} if s == 200 else err(u))
        s, u2 = v2("POST", f"/v2/core/accounts/{aid}", upd)
        print("   same update again:", s, "ok" if s == 200 else err(u2))
        s, o = v2("POST", "/v2/core/account_links", {"account": aid, "use_case": {"type": "account_onboarding", "account_onboarding": {"return_url": "https://accessible.stripe.com/r", "refresh_url": "https://accessible.stripe.com/f"}}})
        print("   link after update:", s, "ok" if s == 200 else err(o))
        for cfg in (["recipient"], []):
            s, o = v2("POST", f"/v2/core/accounts/{aid}/close", {"applied_configurations": cfg})
            print("   close", cfg, s, "" if s == 200 else err(o))
            if s == 200: break
# failed request under a key: replayed or re-executed?
k = "spike-" + uuid.uuid4().hex
import json, base64, urllib.request, urllib.error
def raw(body, key):
    h = {"Authorization": "Basic " + base64.b64encode((sp.KEY + ":").encode()).decode(), "Stripe-Version": "2026-10-28.endive", "Content-Type": "application/json", "Idempotency-Key": key}
    req = urllib.request.Request("https://api.stripe.com/v2/core/accounts", data=json.dumps(body).encode(), method="POST", headers=h)
    try:
        with urllib.request.urlopen(req) as r: return r.status, json.loads(r.read()), dict(r.headers)
    except urllib.error.HTTPError as e: return e.code, json.loads(e.read()), dict(e.headers)
bad = {"identity": IDENT, "configuration": RECIPIENT, "defaults": RESP, "dashboard": "none"}
for i in (1, 2):
    s, o, h = raw(bad, k)
    print(f"v2 failing create #{i} same key:", s, err(o).get("code"), "Idempotent-Replayed=", h.get("Idempotent-Replayed") or h.get("idempotent-replayed"))
s, o, h = raw(dict(bad, contact_email="spike@example.com"), k)
print("v2 same key after the 400, corrected body:", s, err(o) if s != 200 else "created")
if s == 200: print("   close", v2("POST", f"/v2/core/accounts/{o['id']}/close", {"applied_configurations": ["recipient"]})[0])
