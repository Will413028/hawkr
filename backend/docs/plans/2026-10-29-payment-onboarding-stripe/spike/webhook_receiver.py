#!/usr/bin/env python3
"""Tiny Stripe webhook receiver (stdlib only). Secret from env STRIPE_WHSEC_ENV (name below), never logged.
Verifies Stripe-Signature (t=..,v1=..): HMAC-SHA256 of "{t}.{raw body}", constant-time compare, timestamp tolerance 300s.
For every delivery it also logs: altered-body check (must FAIL), stale-timestamp check (must FAIL)."""
import hashlib, hmac, json, os, sys, time
from http.server import BaseHTTPRequestHandler, HTTPServer
SECRET = os.environ["WEBHOOK_SIGNING_SECRET"].encode()
TOL = 300
LOG = sys.argv[1]

def verify(payload: bytes, header: str, now=None, tol=TOL):
    parts = dict(p.split("=", 1) for p in header.split(",") if "=" in p)
    t = parts.get("t"); sigs = [v for k, v in (p.split("=", 1) for p in header.split(",")) if k == "v1"]
    if not t or not sigs: return False, "malformed header"
    if abs((now or time.time()) - int(t)) > tol: return False, "timestamp outside tolerance"
    exp = hmac.new(SECRET, t.encode() + b"." + payload, hashlib.sha256).hexdigest()
    return (any(hmac.compare_digest(exp, s) for s in sigs), "ok" if any(hmac.compare_digest(exp, s) for s in sigs) else "signature mismatch")

class H(BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        hdr = self.headers.get("Stripe-Signature", "")
        ok, why = verify(body, hdr)
        altered = body.replace(b'"livemode": false', b'"livemode": true', 1) if b'"livemode": false' in body else body[:-1] + (b" " if body[-1:] != b" " else b"x")
        ok2, why2 = verify(altered, hdr)
        ok3, why3 = verify(body, hdr, now=time.time() + 1000)
        ev = json.loads(body)
        rec = {"recv": time.strftime("%H:%M:%S"), "event": ev.get("id"), "type": ev.get("type"), "created": ev.get("created"),
               "api_version": ev.get("api_version"), "request_id": (ev.get("request") or {}).get("id"),
               "account": ev.get("account"), "livemode": ev.get("livemode"), "pending_webhooks": ev.get("pending_webhooks"), "verify": ok, "why": why,
               "altered_body_verify": ok2, "altered_why": why2, "stale_ts_verify": ok3, "stale_why": why3,
               "sig_header_parts": sorted(k for k in dict(p.split("=", 1) for p in hdr.split(",")))}
        with open(LOG, "a") as f: f.write(json.dumps(rec) + "\n")
        self.send_response(200 if ok else 400); self.end_headers()
    def log_message(self, *a): pass
HTTPServer(("127.0.0.1", int(sys.argv[2])), H).serve_forever()
