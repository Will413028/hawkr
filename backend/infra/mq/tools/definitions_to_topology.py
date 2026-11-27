#!/usr/bin/env python3
"""RabbitMQ definitions export（dev+prod 兩份）→ canonical topology.json + drift report。
raw export 含 password_hash，只在 scratchpad 使用；輸出 topology.json 不含任何 credential。"""
import argparse
import json

IGNORED_QUEUE_PREFIXES = ("gateway.sse.",)
STRIP_QUEUE_ARGS = ("x-queue-type",)


def load(path):
    with open(path) as f:
        return json.load(f)


def norm_exchanges(defs):
    out = {}
    for e in defs.get("exchanges", []):
        if e["vhost"] != "/" or e["name"].startswith("amq."):
            continue
        out[e["name"]] = {"name": e["name"], "type": e["type"], "durable": e["durable"],
                          "auto_delete": e["auto_delete"], "arguments": e.get("arguments", {})}
    return out


def norm_queues(defs):
    out = {}
    for q in defs.get("queues", []):
        if q["vhost"] != "/" or q["name"].startswith(IGNORED_QUEUE_PREFIXES):
            continue
        args = {k: v for k, v in q.get("arguments", {}).items() if k not in STRIP_QUEUE_ARGS}
        out[q["name"]] = {"name": q["name"], "durable": q["durable"],
                          "auto_delete": q["auto_delete"], "arguments": args}
    return out


def norm_bindings(defs, queues):
    out = {}
    for b in defs.get("bindings", []):
        if b["vhost"] != "/":
            continue
        if b["destination_type"] == "queue" and b["destination"] not in queues:
            continue
        key = f'{b["source"]}|{b["destination_type"]}|{b["destination"]}|{b["routing_key"]}'
        out[key] = {"source": b["source"], "destination": b["destination"],
                    "destination_type": b["destination_type"],
                    "routing_key": b["routing_key"], "arguments": b.get("arguments", {})}
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dev", required=True)
    ap.add_argument("--prod", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--report", required=True)
    a = ap.parse_args()

    ex = {env: norm_exchanges(load(p)) for env, p in (("dev", a.dev), ("prod", a.prod))}
    qu = {env: norm_queues(load(p)) for env, p in (("dev", a.dev), ("prod", a.prod))}
    bi = {env: norm_bindings(load(p), qu[env]) for env, p in (("dev", a.dev), ("prod", a.prod))}

    report = ["# MQ dev/prod drift report", ""]
    for kind, d in (("exchanges", ex), ("queues", qu), ("bindings", bi)):
        only_dev = sorted(set(d["dev"]) - set(d["prod"]))
        only_prod = sorted(set(d["prod"]) - set(d["dev"]))
        mismatch = sorted(k for k in set(d["dev"]) & set(d["prod"]) if d["dev"][k] != d["prod"][k])
        report += [f"## {kind}", f"- dev-only: {only_dev}", f"- prod-only: {only_prod}",
                   f"- 定義不一致: {mismatch}"]
        for k in mismatch:
            report.append(f"  - `{k}`\n    - dev:  {json.dumps(d['dev'][k], ensure_ascii=False, sort_keys=True)}"
                          f"\n    - prod: {json.dumps(d['prod'][k], ensure_ascii=False, sort_keys=True)}")
        report.append("")

    merged_queues = {**qu["dev"], **qu["prod"]}
    queues_plain, queues_json_args = {}, {}
    for name, q in merged_queues.items():
        if any(not isinstance(v, str) for v in q["arguments"].values()):
            queues_json_args[name] = q
        else:
            queues_plain[name] = q

    topo = {
        "exchanges": sorted({**ex["dev"], **ex["prod"]}.values(), key=lambda x: x["name"]),
        "queues": sorted(queues_plain.values(), key=lambda x: x["name"]),
        "queues_json_args": sorted(queues_json_args.values(), key=lambda x: x["name"]),
        "bindings": sorted({**bi["dev"], **bi["prod"]}.values(),
                           key=lambda x: (x["source"], x["destination"], x["routing_key"])),
    }

    non_string = []
    for q in list(topo["queues"]) + list(topo["queues_json_args"]):
        for k, v in q["arguments"].items():
            if not isinstance(v, str):
                non_string.append(f"`{q['name']}`: {k}={v!r} ({type(v).__name__})")
    report += ["## 非 string 的 queue args（分流進 queues_json_args，見 plan Task 2b）"]
    report += [f"- {x}" for x in non_string] if non_string else ["- 無"]

    with open(a.out, "w") as f:
        json.dump(topo, f, indent=2, ensure_ascii=False, sort_keys=True)
        f.write("\n")
    with open(a.report, "w") as f:
        f.write("\n".join(report) + "\n")
    print(f"topology: {len(topo['exchanges'])} exchanges, {len(topo['queues'])} queues "
          f"(+{len(topo['queues_json_args'])} json_args), {len(topo['bindings'])} bindings")


if __name__ == "__main__":
    main()
