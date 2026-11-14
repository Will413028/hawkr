# Step 5: does a v2 Account Link work on 2026-09-30.preview in test mode if payouts is explicitly not requested?
import json, sp, q10_lib
q10_lib.VER = "2026-09-30.preview"
from q10_lib import v2
for label, caps in [("transfers only", {"stripe_transfers": {"requested": True}}),
                    ("payouts false", {"stripe_transfers": {"requested": True}, "payouts": {"requested": False}})]:
    s, a = v2("POST", "/v2/core/accounts", {"contact_email": "spike@example.com", "identity": {"country": "us", "entity_type": "individual"},
        "configuration": {"recipient": {"capabilities": {"stripe_balance": caps}}},
        "defaults": {"responsibilities": {"fees_collector": "application", "losses_collector": "application"}}, "dashboard": "express",
        "include": ["configuration.recipient"]})
    if s != 200: print(label, "create", s, a.get("error")); continue
    sb = a["configuration"]["recipient"]["capabilities"]["stripe_balance"]
    print(label, "create 200 caps:", {k: (v or {}).get("status") for k, v in sb.items()})
    s, o = v2("POST", "/v2/core/account_links", {"account": a["id"], "use_case": {"type": "account_onboarding", "account_onboarding": {"return_url": "https://accessible.stripe.com/r", "refresh_url": "https://accessible.stripe.com/f"}}})
    print(label, "link", s, (o.get("error") or {}).get("message"))
    print("delete", sp.call("DELETE", "/v1/accounts/" + a["id"])[0])
