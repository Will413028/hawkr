# Step 9g (D59, D60): record list-accounts (two pages) and bare-account close responses for the adapter's contract tests (Stripe-Version 2026-10-28.endive).
# Writes one {status, body} JSON per call to argv[1]; the caller scrubs each file into the adapter's testdata. The page size is chosen so the open
# accounts of the sandbox fall on exactly two pages (the adapter itself asks for limit=20).
import base64, json, math, sys, urllib.error, urllib.request
import sp

VER = "2026-10-28.endive"
OUT = sys.argv[1]

def v2(method, path, body=None):
    h = {"Authorization": "Basic " + base64.b64encode((sp.KEY + ":").encode()).decode(), "Stripe-Version": VER}
    if body is not None: h["Content-Type"] = "application/json"
    req = urllib.request.Request("https://api.stripe.com" + path, data=None if body is None else json.dumps(body).encode(), method=method, headers=h)
    try:
        with urllib.request.urlopen(req) as r: s, t = r.status, r.read().decode()
    except urllib.error.HTTPError as e: s, t = e.code, e.read().decode()
    sp._check(json.loads(t)); return s, t

def save(name, s, t):
    with open(f"{OUT}/{name}.json", "w") as f:
        f.write(json.dumps({"status": s, "body": json.loads(t)}, indent=2, ensure_ascii=False))
    print(name, s)

ids = []
for store in ("990025", "990026", "990025"):
    s, t = v2("POST", "/v2/core/accounts", {"metadata": {"store_id": store, "hawkr_namespace": "spike"}}); ids.append(json.loads(t)["id"])
total, page = 0, "/v2/core/accounts?limit=20"
while page:
    o = json.loads(v2("GET", page)[1]); total += len(o["data"]); page = o.get("next_page_url")
limit = math.ceil(total / 2)
print("open accounts", total, "page size", limit)
s, t = v2("GET", f"/v2/core/accounts?limit={limit}"); save("list_accounts_page1", s, t)
nxt = json.loads(t)["next_page_url"]
s, t = v2("GET", nxt); save("list_accounts_page2", s, t)
print("page2 next_page_url is null:", json.loads(t).get("next_page_url") is None)
s, t = v2("POST", f"/v2/core/accounts/{ids[0]}/close", {"applied_configurations": ["recipient"]}); save("close_configs_must_match", s, t)
s, t = v2("POST", f"/v2/core/accounts/{ids[0]}/close", {"applied_configurations": []}); save("close_bare_account", s, t)
for i in ids[1:]: print("close", v2("POST", f"/v2/core/accounts/{i}/close", {"applied_configurations": []})[0])
