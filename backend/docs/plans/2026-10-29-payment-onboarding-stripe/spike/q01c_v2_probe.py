# Q1 follow-up: does Accounts v2 (the API the error message recommends) allow application-collected requirements?
import sp, json, urllib.request, base64
def v2(path, body):
    req = urllib.request.Request("https://api.stripe.com"+path, data=json.dumps(body).encode(), method="POST",
        headers={"Authorization":"Basic "+base64.b64encode((sp.KEY+":").encode()).decode(),
                 "Content-Type":"application/json","Stripe-Version":"2026-04-22.preview"})
    try:
        with urllib.request.urlopen(req) as r: return r.status, json.loads(r.read())
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read())
for c in ["us","tw"]:
    s,o = v2("/v2/core/accounts", {"contact_email":"spike@example.com","identity":{"country":c,"entity_type":"individual"},
        "configuration":{"recipient":{"capabilities":{"stripe_balance":{"stripe_transfers":{"requested":True}}}}},
        "defaults":{"responsibilities":{"fees_collector":"application","losses_collector":"application"}},
        "dashboard":"none"})
    sp.show(f"v2 core account country={c}", s, o, None if s!=200 else ["id","identity","configuration","defaults","dashboard","requirements"])
    if s==200:
        sp.call("DELETE","/v1/accounts/"+o["id"])
