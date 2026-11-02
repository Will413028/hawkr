# Step 5: record raw responses for the adapter's contract tests (Stripe-Version 2026-09-30.preview).
# Writes one JSON per call to argv[1]; the caller scrubs each file into outputs/fixtures/.
import base64, json, sys, time, urllib.error, urllib.request, uuid
import sp

VER = "2026-09-30.preview"
OUT = sys.argv[1]
INC = ["configuration.recipient", "requirements", "identity", "defaults"]


def v2(method, path, body=None, idem=None):
    h = {"Authorization": "Basic " + base64.b64encode((sp.KEY + ":").encode()).decode(), "Stripe-Version": VER}
    if body is not None:
        h["Content-Type"] = "application/json"
    if idem:
        h["Idempotency-Key"] = idem
    req = urllib.request.Request("https://api.stripe.com" + path, data=None if body is None else json.dumps(body).encode(), method=method, headers=h)
    try:
        with urllib.request.urlopen(req) as r:
            s, t = r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        s, t = e.code, e.read().decode()
    sp._check(json.loads(t))
    return s, t


def save(name, s, t):
    with open(f"{OUT}/{name}.json", "w") as f:
        f.write(json.dumps({"status": s, "body": json.loads(t)}, indent=2, ensure_ascii=False))
    print(name, s)


def retrieve(aid):
    return v2("GET", f"/v2/core/accounts/{aid}?" + "&".join("include=" + i for i in INC))


ids = []
for mode, dash in [("platform", "none"), ("provider", "express")]:
    b = {"contact_email": "spike@example.com", "display_name": "Spike Store", "identity": {"country": "us", "entity_type": "individual"},
         "configuration": {"recipient": {"capabilities": {"stripe_balance": {"stripe_transfers": {"requested": True}}}}},
         "defaults": {"responsibilities": {"fees_collector": "application", "losses_collector": "application"}}, "dashboard": dash, "include": INC}
    k = "spike-" + uuid.uuid4().hex
    s, t = v2("POST", "/v2/core/accounts", b, k); save(f"create_{mode}", s, t)
    aid = json.loads(t)["id"]; ids.append(aid)
    s, t = v2("POST", "/v2/core/accounts", dict(b, display_name="x"), k); save(f"create_{mode}_idem_conflict", s, t)
    s, t = retrieve(aid); save(f"retrieve_{mode}_new", s, t)
    s, t = v2("POST", "/v2/core/account_links", {"account": aid, "use_case": {"type": "account_onboarding", "account_onboarding": {
        "return_url": "https://example.com/r", "refresh_url": "https://example.com/f"}}})
    save(f"account_link_{mode}", s, t)
    s, o, _ = sp.call("POST", "/v1/account_sessions", {"account": aid, "components": {"account_onboarding": {"enabled": True}}})
    save(f"account_session_{mode}", s, json.dumps(o))

s, t = v2("GET", "/v2/core/accounts/acct_doesnotexist000"); save("retrieve_not_found", s, t)

aid = ids[0]
s, t = v2("POST", f"/v2/core/accounts/{aid}", {"identity": {"individual": {
    "given_name": "Jenny", "surname": "Rosen", "email": "jenny@example.com", "phone": "+12015550123",
    "date_of_birth": {"day": 1, "month": 1, "year": 1990},
    "address": {"line1": "address_full_match", "city": "San Francisco", "state": "CA", "postal_code": "94111", "country": "us"},
    "id_numbers": [{"type": "us_ssn_last_4", "value": "0000"}]},
    "attestations": {"terms_of_service": {"account": {"date": "2026-11-02T00:00:00Z", "ip": "8.8.8.8"}}}},
    "defaults": {"profile": {"business_url": "https://example.com"}}, "include": INC})
save("update_platform_identity", s, t)
s, o, _ = sp.call("POST", f"/v1/accounts/{aid}/external_accounts", {"external_account": {
    "object": "bank_account", "country": "US", "currency": "usd", "routing_number": "110000000", "account_number": "000123456789"}})
print("bank", s, (o.get("error") or {}).get("message"))
for _ in range(12):
    time.sleep(10)
    s, t = retrieve(aid)
    st = json.loads(t)["configuration"]["recipient"]["capabilities"]["stripe_balance"]["stripe_transfers"]["status"]
    print("poll", st)
    if st == "active":
        break
save("retrieve_platform_after_submit", s, t)
for i in ids:
    print("delete", sp.call("DELETE", "/v1/accounts/" + i)[0])
