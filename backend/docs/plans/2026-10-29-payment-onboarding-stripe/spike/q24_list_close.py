# Step 9g (D59) Q24-Q26: does v2 list include bare shells with metadata; how closed accounts show; applied_configurations for closing a shell; close error with balance.
import uuid, json, sp, q10_lib
q10_lib.VER = "2026-10-28.endive"
from q10_lib import v2
def err(o): e = o.get("error") or {}; return {"type": e.get("type"), "code": e.get("code"), "message": e.get("message")}
sid = "q24-" + uuid.uuid4().hex[:8]
s, a = v2("POST", "/v2/core/accounts", {"metadata": {"store_id": sid}}); shell = a["id"]
print("Q24 create shell", s, "applied=", a.get("applied_configurations"))
s, b = v2("POST", "/v2/core/accounts", {"metadata": {"store_id": sid}}); closed_one = b["id"]
print("Q24 second shell same store_id", s)
def scan(params=""):
    found, page, pages = {}, f"/v2/core/accounts?limit=20{params}", 0
    while page and pages < 200:
        s, o = v2("GET", page); pages += 1
        if s != 200: print("list", params, s, err(o)); return None
        for x in o.get("data", []):
            if x["id"] in (shell, closed_one): found[x["id"]] = {"metadata": x.get("metadata"), "closed": x.get("closed"), "applied": x.get("applied_configurations"), "keys": sorted(x.keys())}
        page = o.get("next_page_url")
    print(f"list{params or ' (default)'} pages={pages}", json.dumps({("shell" if k == shell else "closed"): v for k, v in found.items()}, ensure_ascii=False))
s, o = v2("POST", f"/v2/core/accounts/{closed_one}/close", {"applied_configurations": []}); print("close second shell []", s, o.get("closed"))
scan(); scan("&closed=true"); scan("&applied_configurations=recipient")
print("Q25 close shell ['recipient']")
s, o = v2("POST", f"/v2/core/accounts/{shell}/close", {"applied_configurations": ["recipient"]}); print(" ", s, err(o) if s != 200 else o.get("closed"))
if s != 200:
    s, o = v2("GET", f"/v2/core/accounts/{shell}"); print("  retrieve applied=", o.get("applied_configurations"))
    s, o = v2("POST", f"/v2/core/accounts/{shell}/close", {}); print("  close {} (omitted)", s, err(o) if s != 200 else o.get("closed"))
    if s != 200:
        s, o = v2("POST", f"/v2/core/accounts/{shell}/close", {"applied_configurations": []}); print("  close []", s, err(o) if s != 200 else o.get("closed"))
print("Q26 balance then close")
s, r = v2("POST", "/v2/core/accounts", {"metadata": {"store_id": sid}, "contact_email": "spike@example.com", "identity": {"country": "us", "entity_type": "individual"},
    "configuration": {"recipient": {"capabilities": {"stripe_balance": {"stripe_transfers": {"requested": True}}}}},
    "defaults": {"responsibilities": {"fees_collector": "application", "losses_collector": "application"}}, "dashboard": "none"})
rid = r.get("id"); print(" create recipient", s, err(r) if s != 200 else "")
s, c, _ = sp.call("POST", "/v1/charges", {"amount": 2000, "currency": "usd", "source": "tok_bypassPending"}); print(" platform charge", s, (c.get("error") or {}).get("code"))
s, t, _ = sp.call("POST", "/v1/transfers", {"amount": 500, "currency": "usd", "destination": rid}); print(" transfer", s, (t.get("error") or {}).get("code"), (t.get("error") or {}).get("message"))
s, o = v2("POST", f"/v2/core/accounts/{rid}/close", {"applied_configurations": ["recipient"]}); print(" close with balance", s, err(o) if s != 200 else o.get("closed"))
if s != 200:
    s, o = v2("GET", f"/v2/core/accounts/{rid}"); print(" retrieve applied=", o.get("applied_configurations"))
