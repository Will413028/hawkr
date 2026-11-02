import sp, json, urllib.request, base64
VER="2026-04-22.preview"
def v2(method, path, body=None, ctx=None):
    h={"Authorization":"Basic "+base64.b64encode((sp.KEY+":").encode()).decode(),"Stripe-Version":VER}
    if body is not None: h["Content-Type"]="application/json"
    if ctx: h["Stripe-Context"]=ctx
    req=urllib.request.Request("https://api.stripe.com"+path,data=None if body is None else json.dumps(body).encode(),method=method,headers=h)
    try:
        with urllib.request.urlopen(req) as r: s,o=r.status,json.loads(r.read())
    except urllib.error.HTTPError as e: s,o=e.code,json.loads(e.read())
    sp._check(o); return s,o
