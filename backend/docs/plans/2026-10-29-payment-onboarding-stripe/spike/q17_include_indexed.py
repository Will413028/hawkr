# Step 9b: stripe-go sends v2 GET includes as include[0]=..&include[1]=..; does the API accept that form?
import sp, q10_lib
q10_lib.VER = "2026-10-28.endive"
from q10_lib import v2
s, a = v2("POST", "/v2/core/accounts", {"contact_email": "spike@example.com", "identity": {"country": "us", "entity_type": "individual"},
    "configuration": {"recipient": {"capabilities": {"stripe_balance": {"stripe_transfers": {"requested": True}}}}},
    "defaults": {"responsibilities": {"fees_collector": "application", "losses_collector": "application"}}, "dashboard": "none"})
print("create", s); aid = a["id"]
for label, q in [("repeated", "include=configuration.recipient&include=requirements"),
                 ("indexed", "include%5B0%5D=configuration.recipient&include%5B1%5D=requirements"),
                 ("none", "")]:
    s, o = v2("GET", f"/v2/core/accounts/{aid}?{q}")
    print(label, s, {"recipient": (o.get("configuration") or {}).get("recipient") is not None, "requirements": o.get("requirements") is not None} if s == 200 else o.get("error"))
print("close", v2("POST", f"/v2/core/accounts/{aid}/close", {"applied_configurations": ["recipient"]})[0])
