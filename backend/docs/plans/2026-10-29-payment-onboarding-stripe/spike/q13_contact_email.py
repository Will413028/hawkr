# Step 5b: does a v2 account need contact_email per dashboard mode (2026-09-30.preview, sandbox)?
import sp, q10_lib
q10_lib.VER = "2026-09-30.preview"
from q10_lib import v2
for dash in ["none", "express"]:
    s, a = v2("POST", "/v2/core/accounts", {"identity": {"country": "us", "entity_type": "individual"},
        "configuration": {"recipient": {"capabilities": {"stripe_balance": {"stripe_transfers": {"requested": True}}}}},
        "defaults": {"responsibilities": {"fees_collector": "application", "losses_collector": "application"}}, "dashboard": dash})
    print(dash, "no contact_email:", s, (a.get("error") or {}).get("message"))
    if s == 200:
        print("delete", sp.call("DELETE", "/v1/accounts/" + a["id"])[0])
