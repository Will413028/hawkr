# Step 5: Accounts v2 recipient (US), idempotent replay, both collection modes, bank fingerprint, rename, account link.
import sp, json, urllib.request, base64, uuid, hashlib
VER = "2026-04-22.preview"
def v2(method, path, body=None, idem=None):
    h = {"Authorization":"Basic "+base64.b64encode((sp.KEY+":").encode()).decode(),"Stripe-Version":VER}
    if body is not None: h["Content-Type"]="application/json"
    if idem: h["Idempotency-Key"]=idem
    req = urllib.request.Request("https://api.stripe.com"+path, data=None if body is None else json.dumps(body).encode(), method=method, headers=h)
    try:
        with urllib.request.urlopen(req) as r: s,o = r.status, json.loads(r.read())
    except urllib.error.HTTPError as e: s,o = e.code, json.loads(e.read())
    sp._check(o); return s,o
def tag(x): return "fp_"+hashlib.sha256(x.encode()).hexdigest()[:4] if x else None
def body(collector):
    b = {"contact_email":"spike@example.com","display_name":"Spike Store","identity":{"country":"us","entity_type":"individual"},
         "configuration":{"recipient":{"capabilities":{"stripe_balance":{"stripe_transfers":{"requested":True}}}}},
         "include":["configuration.recipient","requirements","identity","defaults"]}
    if collector=="application":
        b["defaults"]={"responsibilities":{"fees_collector":"application","losses_collector":"application"}}; b["dashboard"]="none"
    else:
        b["defaults"]={"responsibilities":{"fees_collector":"application","losses_collector":"application"}}; b["dashboard"]="express"
    return b
bank={"object":"bank_account","country":"US","currency":"usd","routing_number":"110000000","account_number":"000123456789"}
made=[]
for mode in ["application","stripe"]:
    k="spike-"+uuid.uuid4().hex
    s1,a1=v2("POST","/v2/core/accounts",body(mode),k)
    if s1!=200: sp.show(f"[{mode}] create ERROR",s1,a1); continue
    made.append(a1["id"])
    s2,a2=v2("POST","/v2/core/accounts",body(mode),k)
    print(f"[{mode}] create {s1}; replay {s2} same id: {a1['id']==a2.get('id')}")
    b2=body(mode); b2["display_name"]="Other"
    s3,a3=v2("POST","/v2/core/accounts",b2,k)
    print(f"[{mode}] replay with different body: {s3} {(a3.get('error') or {}).get('code')}")
    sp.show(f"[{mode}] account", s1, a1, ["object","dashboard","defaults"])
    s,o,_=sp.call("POST",f"/v1/accounts/{a1['id']}/external_accounts",{"external_account":bank})
    print(f"[{mode}] attach bank via v1: {s} {(o.get('error') or {}).get('message')}")
    s,o,_=sp.call("GET",f"/v1/accounts/{a1['id']}/external_accounts",{"object":"bank_account"})
    d=(o.get("data") or [{}])
    print(f"[{mode}] list external_accounts: {s} fingerprint present: {'fingerprint' in d[0] if d else None} tag={tag(d[0].get('fingerprint')) if d else None} err={(o.get('error') or {}).get('message')}")
    s,o=v2("POST",f"/v2/core/accounts/{a1['id']}",{"display_name":"Renamed Store"})
    print(f"[{mode}] rename: {s} display_name={o.get('display_name')} err={(o.get('error') or {}).get('code')}")
    s,o=v2("POST","/v2/core/account_links",{"account":a1["id"],"use_case":{"type":"account_onboarding","account_onboarding":{"configurations":["recipient"],"return_url":"https://example.com/r","refresh_url":"https://example.com/f"}}})
    print(f"[{mode}] account_link: {s} has url: {bool(o.get('url'))} expires_at={o.get('expires_at')} err={o.get('error')}")
    s,o,_=sp.call("POST","/v1/account_sessions",{"account":a1["id"],"components":{"account_onboarding":{"enabled":True}}})
    print(f"[{mode}] account_session: {s} has secret: {bool(o.get('client_secret'))} err={(o.get('error') or {}).get('message')}")
for i in made:
    s,o=v2("POST",f"/v2/core/accounts/{i}/close",{})
    print("close",s,(o.get('error') or {}).get('code'))
    if s!=200: print("v1 delete",sp.call("DELETE","/v1/accounts/"+i)[0])
