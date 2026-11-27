"""R2 presigned PUT probe: which headers can be bound into the signature.
Reads R2_* from the environment; prints only status codes and error codes."""
import datetime, hashlib, hmac, http.client, os, re, urllib.parse, uuid

ACC = os.environ["R2_ACCOUNT_ID"]
BUCKET = os.environ["R2_BUCKET"]
AK = os.environ["R2_ACCESS_KEY_ID"]
SK = os.environ["R2_SECRET_ACCESS_KEY"]
HOST = f"{ACC}.r2.cloudflarestorage.com"
REGION = "auto"


def _h(key, msg):
    return hmac.new(key, msg.encode(), hashlib.sha256).digest()


def presign(method, key, signed, expires=300):
    """signed: dict of extra headers to bind (lower-case names)."""
    now = datetime.datetime.now(datetime.timezone.utc)
    amz = now.strftime("%Y%m%dT%H%M%SZ")
    day = now.strftime("%Y%m%d")
    scope = f"{day}/{REGION}/s3/aws4_request"
    headers = {"host": HOST, **signed}
    names = ";".join(sorted(headers))
    q = {
        "X-Amz-Algorithm": "AWS4-HMAC-SHA256",
        "X-Amz-Credential": f"{AK}/{scope}",
        "X-Amz-Date": amz,
        "X-Amz-Expires": str(expires),
        "X-Amz-SignedHeaders": names,
    }
    cq = "&".join(f"{urllib.parse.quote(k, safe='')}={urllib.parse.quote(v, safe='')}" for k, v in sorted(q.items()))
    path = "/" + BUCKET + "/" + urllib.parse.quote(key)
    ch = "".join(f"{k}:{headers[k]}\n" for k in sorted(headers))
    creq = "\n".join([method, path, cq, ch, names, "UNSIGNED-PAYLOAD"])
    sts = "\n".join(["AWS4-HMAC-SHA256", amz, scope, hashlib.sha256(creq.encode()).hexdigest()])
    k = _h(_h(_h(_h(("AWS4" + SK).encode(), day), REGION), "s3"), "aws4_request")
    sig = hmac.new(k, sts.encode(), hashlib.sha256).hexdigest()
    return path + "?" + cq + "&X-Amz-Signature=" + sig


def send(method, url, body=b"", headers=None):
    c = http.client.HTTPSConnection(HOST, timeout=30)
    c.request(method, url, body=body, headers=headers or {})
    r = c.getresponse()
    text = r.read().decode("utf-8", "replace")
    code = re.search(r"<Code>([^<]+)</Code>", text)
    c.close()
    return r.status, (code.group(1) if code else "")


def case(name, status_code):
    print(f"{name}: HTTP {status_code[0]} {status_code[1]}".rstrip())


run = uuid.uuid4().hex[:8]
k = lambda n: f"tmp/probe-{run}/{n}"
PNG = "image/png"

case("0 control (only host signed), 2000 bytes text/plain",
     send("PUT", presign("PUT", k("c0"), {}), b"x" * 2000, {"Content-Type": "text/plain"}))

s = {"content-length": "1000", "content-type": PNG}
case("1a signed len=1000 type=png, send 1000 png",
     send("PUT", presign("PUT", k("c1a"), s), b"x" * 1000, {"Content-Type": PNG}))
case("1b signed len=1000 type=png, send 2000 png",
     send("PUT", presign("PUT", k("c1b"), s), b"x" * 2000, {"Content-Type": PNG}))
case("1c signed len=1000 type=png, send 999 png",
     send("PUT", presign("PUT", k("c1c"), s), b"x" * 999, {"Content-Type": PNG}))
case("1d signed len=1000 type=png, send 1000 text/plain",
     send("PUT", presign("PUT", k("c1d"), s), b"x" * 1000, {"Content-Type": "text/plain"}))
case("1e signed type=png only, send 5000 png",
     send("PUT", presign("PUT", k("c1e"), {"content-type": PNG}), b"x" * 5000, {"Content-Type": PNG}))

for n in ("c0", "c1a", "c1b", "c1c", "c1d", "c1e"):
    case(f"  HEAD {n}", send("HEAD", presign("HEAD", k(n), {})))

s = {"if-none-match": "*", "content-type": PNG}
case("2a signed if-none-match:*, first PUT",
     send("PUT", presign("PUT", k("c2"), s), b"a" * 10, {"Content-Type": PNG, "If-None-Match": "*"}))
case("2b signed if-none-match:*, second PUT same key",
     send("PUT", presign("PUT", k("c2"), s), b"b" * 10, {"Content-Type": PNG, "If-None-Match": "*"}))
case("2c signed if-none-match:*, client omits the header",
     send("PUT", presign("PUT", k("c2c"), s), b"c" * 10, {"Content-Type": PNG}))

import base64
body = b"m" * 100
good = base64.b64encode(hashlib.md5(body).digest()).decode()
s = {"content-md5": good, "content-type": PNG}
case("3a signed content-md5, matching body",
     send("PUT", presign("PUT", k("c3a"), s), body, {"Content-Type": PNG, "Content-MD5": good}))
case("3b signed content-md5, different body same length",
     send("PUT", presign("PUT", k("c3b"), s), b"n" * 100, {"Content-Type": PNG, "Content-MD5": good}))

for n in ("c0", "c1a", "c1b", "c1c", "c1d", "c1e", "c2", "c2c", "c3a", "c3b", ):
    send("DELETE", presign("DELETE", k(n), {}))
print("cleanup done; run", run)
