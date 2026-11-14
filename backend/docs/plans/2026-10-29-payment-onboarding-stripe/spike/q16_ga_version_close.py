# Step 9b: do the adapter's calls work on the SDK-pinned GA version (stripe-go v87.1.0, 2026-10-28.endive)
# in the sandbox, and how does POST /v2/core/accounts/{id}/close behave (first call, repeat, unknown id)?
import json, sp, q10_lib
VER = "2026-10-28.endive"
q10_lib.VER = VER
from q10_lib import v2

def err(o):
    e = o.get("error") or {}
    return {k: e.get(k) for k in ("code", "type", "message") if e.get(k)}

def caps(a):
    sb = (((a.get("configuration") or {}).get("recipient") or {}).get("capabilities") or {}).get("stripe_balance") or {}
    return {k: (v or {}).get("status") for k, v in sb.items()}

for dashboard in ("express", "none"):
    print(f"=== dashboard={dashboard} version={VER}")
    s, a = v2("POST", "/v2/core/accounts", {"contact_email": "spike@example.com",
        "identity": {"country": "us", "entity_type": "individual"},
        "configuration": {"recipient": {"capabilities": {"stripe_balance": {"stripe_transfers": {"requested": True}}}}},
        "defaults": {"responsibilities": {"fees_collector": "application", "losses_collector": "application"}},
        "dashboard": dashboard, "include": ["configuration.recipient", "requirements"]})
    print("create", s, caps(a) if s == 200 else err(a))
    if s != 200: continue
    aid = a["id"]
    s, o = v2("POST", "/v2/core/account_links", {"account": aid, "use_case": {"type": "account_onboarding",
        "account_onboarding": {"return_url": "https://accessible.stripe.com/r", "refresh_url": "https://accessible.stripe.com/f"}}})
    print("account_link", s, sorted(o.keys()) if s == 200 else err(o))
    s, o = v2("GET", f"/v2/core/accounts/{aid}?include=configuration.recipient&include=requirements")
    print("retrieve", s, {"caps": caps(o), "requirements_keys": sorted((o.get("requirements") or {}).keys()), "closed": o.get("closed")} if s == 200 else err(o))
    s, o, _ = sp.call("POST", "/v1/account_sessions", {"account": aid, "components": {"account_onboarding": {"enabled": True}}}, headers={"Stripe-Version": VER})
    print("v1 account_sessions", s, sorted(o.keys()) if s == 200 else err(o))
    s, o, _ = sp.call("GET", f"/v1/accounts/{aid}/external_accounts", {"object": "bank_account", "limit": 100}, headers={"Stripe-Version": VER})
    print("v1 external_accounts", s, {"object": o.get("object"), "count": len(o.get("data") or [])} if s == 200 else err(o))
    s, o = v2("POST", f"/v2/core/accounts/{aid}/close", {})
    print("close without applied_configurations", s, {"closed": o.get("closed")} if s == 200 else err(o))
    s, o = v2("POST", f"/v2/core/accounts/{aid}/close", {"applied_configurations": ["recipient"]})
    print("close", s, {"closed": o.get("closed"), "applied_configurations": o.get("applied_configurations")} if s == 200 else err(o))
    s, o = v2("POST", f"/v2/core/accounts/{aid}/close", {"applied_configurations": ["recipient"]})
    print("close again", s, {"closed": o.get("closed")} if s == 200 else err(o))
    s, o = v2("GET", f"/v2/core/accounts/{aid}?include=configuration.recipient")
    print("retrieve after close", s, {"closed": o.get("closed"), "caps": caps(o)} if s == 200 else err(o))
    s, o = v2("POST", "/v2/core/account_links", {"account": aid, "use_case": {"type": "account_onboarding",
        "account_onboarding": {"return_url": "https://accessible.stripe.com/r", "refresh_url": "https://accessible.stripe.com/f"}}})
    print("account_link after close", s, "ok" if s == 200 else err(o))
s, o = v2("POST", "/v2/core/accounts/acct_doesnotexist000/close", {"applied_configurations": ["recipient"]})
print("close unknown id", s, err(o))
