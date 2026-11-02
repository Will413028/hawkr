"""Tiny Stripe test-mode client for the spike. Key comes from env STRIPE_SECRET_KEY
(never printed). Aborts if any response shows livemode true."""
import base64, json, os, sys, urllib.error, urllib.parse, urllib.request

BASE = "https://api.stripe.com"
KEY = os.environ["STRIPE_SECRET_KEY"]
if not (KEY.startswith("sk_" "test_") or KEY.startswith("rk_" "test_")):
    sys.exit("refusing: not a test key")

def flat(d, prefix=""):
    out = []
    if isinstance(d, dict):
        for k, v in d.items():
            out += flat(v, f"{prefix}[{k}]" if prefix else k)
    elif isinstance(d, (list, tuple)):
        for i, v in enumerate(d):
            out += flat(v, f"{prefix}[{i}]")
    elif isinstance(d, bool):
        out.append((prefix, "true" if d else "false"))
    elif d is not None:
        out.append((prefix, str(d)))
    return out

def _check(obj):
    if isinstance(obj, dict):
        if obj.get("livemode") is True:
            sys.exit("LIVEMODE TRUE - STOP")
        for v in obj.values(): _check(v)
    elif isinstance(obj, list):
        for v in obj: _check(v)

def call(method, path, data=None, idem=None, files=None, headers=None, host=BASE):
    h = {"Authorization": "Basic " + base64.b64encode((KEY + ":").encode()).decode()}
    if idem: h["Idempotency-Key"] = idem
    if headers: h.update(headers)
    body = None
    if files:
        b = "----spike"
        parts = []
        for k, v in flat(data or {}):
            parts.append(f'--{b}\r\nContent-Disposition: form-data; name="{k}"\r\n\r\n{v}\r\n'.encode())
        for k, (fn, content, ct) in files.items():
            parts.append(f'--{b}\r\nContent-Disposition: form-data; name="{k}"; filename="{fn}"\r\nContent-Type: {ct}\r\n\r\n'.encode() + content + b"\r\n")
        parts.append(f"--{b}--\r\n".encode())
        body = b"".join(parts)
        h["Content-Type"] = "multipart/form-data; boundary=" + b
    elif data is not None and method != "GET":
        body = urllib.parse.urlencode(flat(data)).encode()
        h["Content-Type"] = "application/x-www-form-urlencoded"
    url = host + path
    if method == "GET" and data:
        url += "?" + urllib.parse.urlencode(flat(data))
    req = urllib.request.Request(url, data=body, headers=h, method=method)
    try:
        with urllib.request.urlopen(req) as r:
            status, text, rh = r.status, r.read().decode(), dict(r.headers)
    except urllib.error.HTTPError as e:
        status, text, rh = e.code, e.read().decode(), dict(e.headers)
    try:
        obj = json.loads(text)
    except Exception:
        obj = {"_raw": text}
    _check(obj)
    return status, obj, rh

def show(title, status, obj, keys=None):
    print(f"--- {title} (HTTP {status})")
    if keys and isinstance(obj, dict):
        obj = {k: obj.get(k) for k in keys}
    print(json.dumps(obj, indent=2, ensure_ascii=False))

def rq(a):
    r = a.get("requirements") or {}
    return {k: r.get(k) for k in ("currently_due", "eventually_due", "past_due", "pending_verification", "errors", "disabled_reason", "current_deadline")}

def state(a):
    return {"charges_enabled": a.get("charges_enabled"), "payouts_enabled": a.get("payouts_enabled"),
            "details_submitted": a.get("details_submitted"), "requirements": rq(a)}
