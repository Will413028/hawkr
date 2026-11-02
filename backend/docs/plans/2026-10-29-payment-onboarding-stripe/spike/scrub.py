#!/usr/bin/env python3
"""Mask secrets, non-acct/evt identifiers and PII in text read from stdin.

Usage: some_script | python3 scrub.py > outputs/x.txt
Ids are replaced by <prefix>_<MASKED:4hex> where the 4hex is a stable hash,
so equality between two masked values stays visible without revealing them.
"""
import hashlib
import re
import sys

def tag(v):
    return hashlib.sha256(v.encode()).hexdigest()[:4]

# secrets: never keep any part
SECRET = re.compile(r"\b(?:sk|rk|pk)" r"_(?:test|live)_[A-Za-z0-9_]+|\bwh" r"sec_[A-Za-z0-9_]+")
# ids: keep only acct_ and evt_
ID = re.compile(r"\b(?!acct_|evt_|file_identity)(ba|btok|file|person|tok|req|link|acctsess|cs|pi|ch|txn|po|tr|fa|ipi|src|pm|cus|seti|si|ca|ek)_[A-Za-z0-9]{8,}")
UUID = re.compile(r"\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b")
CSID = re.compile(r"\b(?:client_secret|ccs|accs_secret)_[A-Za-z0-9_]+")
EMAIL = re.compile(r"[\w.+-]+@[\w-]+(?:\.[\w-]+)+")
URLTOK = re.compile(r"(https://connect\.stripe\.com/[A-Za-z0-9_/.-]*?)/[A-Za-z0-9_-]{8,}")
# JSON keys whose string/number values are personal data
PII_KEYS = (
    "first_name|last_name|full_name|name|email|phone|line1|line2|city|state|postal_code|"
    "id_number|ssn_last_4|id_number_provided|account_holder_name|account_number|routing_number|"
    "last4|fingerprint_x|ip|user_agent|url|business_profile_url|statement_descriptor|"
    "idempotency_key|Idempotency-Key|secret|expires_at_x"
)
KEYVAL = re.compile(r'("(?:%s)"\s*:\s*)("(?:[^"\\]|\\.)*"|\d+)' % PII_KEYS)
DOBOBJ = re.compile(r'("dob"\s*:\s*\{)[^}]*\}')
ANYNAME = re.compile(r'"(day|month|year)"\s*:\s*\d+')
HEADER = re.compile(r"(?im)^((?:idempotency-key|stripe-signature|authorization|request-id)\s*[:=]\s*).*$")

def sub_id(m):
    return "%s_<MASKED:%s>" % (m.group(1), tag(m.group(0)))

def scrub(s):
    s = SECRET.sub("<SECRET-MASKED>", s)
    s = ID.sub(sub_id, s)
    s = UUID.sub("<UUID-MASKED>", s)
    s = CSID.sub("<CLIENT-SECRET-MASKED>", s)
    s = EMAIL.sub("<EMAIL-MASKED>", s)
    s = URLTOK.sub(r"\1/<TOKEN-MASKED>", s)
    s = DOBOBJ.sub(r'\1"day": "<MASKED>", "month": "<MASKED>", "year": "<MASKED>"}', s)
    s = KEYVAL.sub(lambda m: m.group(1) + '"<MASKED>"', s)
    s = HEADER.sub(lambda m: m.group(1) + "<MASKED>", s)
    # t=<unix>,v1=<hex> signature components
    s = re.sub(r"\bv1=[0-9a-f]{16,}", "v1=<SIG-MASKED>", s)
    s = re.sub(r"\bv0=[0-9a-f]{16,}", "v0=<SIG-MASKED>", s)
    # DOB-like and long digit runs that look like ID/account numbers
    s = re.sub(r"\b\d{4}-\d{2}-\d{2}\b(?=.*dob)", "<DOB-MASKED>", s)
    s = re.sub(r"\b0{6,}\d*\b", "<IDNUM-MASKED>", s)
    return s

if __name__ == "__main__":
    sys.stdout.write(scrub(sys.stdin.read()))
