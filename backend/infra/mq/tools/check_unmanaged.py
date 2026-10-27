#!/usr/bin/env python3
# infra/mq/tools/check_unmanaged.py
"""terraform plan 看不到 unmanaged 資源（手動 SSH 建的實體是盲區），nightly 用此補上。
已知盲區：只查 queue/exchange，不查 unmanaged binding（tf plan 只看 managed、本腳本不看
binding——手動加的野 binding 兩邊都抓不到；Phase 1 接受，Phase 2 可補 /api/bindings 比對）。"""
import argparse
import base64
import json
import sys
import urllib.request

IGNORED_QUEUE_PREFIXES = ("gateway.sse.",)


def api(endpoint, user, pw, path):
    req = urllib.request.Request(f"{endpoint}/api/{path}")
    req.add_header("Authorization",
                   "Basic " + base64.b64encode(f"{user}:{pw}".encode()).decode())
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.load(r)


def main():
    ap = argparse.ArgumentParser()
    for f in ("endpoint", "user", "password", "topology"):
        ap.add_argument(f"--{f}", required=True)
    a = ap.parse_args()
    topo = json.load(open(a.topology))
    known_q = {q["name"] for q in topo["queues"]} | {q["name"] for q in topo.get("queues_json_args", [])}
    known_e = {e["name"] for e in topo["exchanges"]}
    live_q = {q["name"] for q in api(a.endpoint, a.user, a.password, "queues/%2F")
              if not q["name"].startswith(IGNORED_QUEUE_PREFIXES)}
    live_e = {e["name"] for e in api(a.endpoint, a.user, a.password, "exchanges/%2F")
              if e["name"] and not e["name"].startswith("amq.")}
    unmanaged = [("exchange", n) for n in sorted(live_e - known_e)] + \
                [("queue", n) for n in sorted(live_q - known_q)]
    if unmanaged:
        print("⚠️ broker 上存在 topology.yaml 宣告之外的實體（手動建立？）:")
        for kind, n in unmanaged:
            print(f"  {kind}: {n}")
        sys.exit(1)
    print("✅ 無 unmanaged 實體")


if __name__ == "__main__":
    main()
