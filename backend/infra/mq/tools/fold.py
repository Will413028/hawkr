"""一次性摺疊器：攤平的 topology.json → 宣告式 topology.yaml。

P1 遷移後保留作災難復原用（broker dump → definitions_to_topology.py → fold.py）。
正常維護流程不需要跑這支。
"""

import argparse
import json
import sys


def fold(flat):
    exchanges = {e["name"]: e for e in flat["exchanges"]}
    queues = {q["name"]: q for q in flat["queues"]}
    json_args = {q["name"]: q for q in flat.get("queues_json_args", [])}

    # 衍生物辨識：.retry fanout exchange 對應的 base queue 即 Go retry queue
    retry_bases = {
        n[: -len(".retry")]
        for n, e in exchanges.items()
        if n.endswith(".retry") and e["type"] == "fanout"
    }

    bindings_by_dest = {}
    for b in flat["bindings"]:
        bindings_by_dest.setdefault(b["destination"], []).append(b)

    # 被 policy 展開吃掉、不該獨立出現在 YAML 的資源
    derived_exchanges = {f"{q}.retry" for q in retry_bases}
    derived_queues = set()

    out_queues = []
    for name, q in list(queues.items()) + list(json_args.items()):
        if name.endswith(".dlq"):
            base = name[: -len(".dlq")]
            if base in queues or base in retry_bases:
                derived_queues.add(name)
                continue  # 由 base queue 的 policy 展開產生

        entry = {"name": name}
        args = dict(q["arguments"])

        own = [
            b
            for b in bindings_by_dest.get(name, [])
            if not b["source"].endswith(".retry")
        ]

        if name in retry_bases:
            entry["retry"] = {"max_attempts": 3}
            derived_queues.add(f"{name}.dlq")
        elif "x-dead-letter-exchange" in args:
            via = args.pop("x-dead-letter-exchange")
            dlq = {"via": via}
            rk = args.pop("x-dead-letter-routing-key", None)
            own_dlq = f"{name}.dlq"
            if own_dlq in queues:
                derived_queues.add(own_dlq)
            else:
                # dead-letter 到共用 DLQ：需顯式指名目標
                target = _shared_dlq_target(via, rk, bindings_by_dest)
                dlq["queue"] = target
            if rk is not None:
                dlq["routing_key"] = rk
            entry["dlq"] = dlq

        if own:
            entry["bindings"] = [
                {"exchange": b["source"], "key": b["routing_key"]} for b in own
            ]
        if name in json_args:
            entry["args_json"] = True
        if args:
            entry["arguments"] = args

        out_queues.append(entry)

    out_queues = [q for q in out_queues if q["name"] not in derived_queues]

    return {
        "exchanges": [
            {"name": n, "type": e["type"]}
            for n, e in sorted(exchanges.items())
            if n not in derived_exchanges
        ],
        "queues": sorted(out_queues, key=lambda q: q["name"]),
    }


def _shared_dlq_target(via, routing_key, bindings_by_dest):
    if via == "":
        # 空字串代表 RabbitMQ 的 default exchange：訊息直接依 routing key
        # 送進同名 queue，不會有對應的 binding 條目可查。
        if routing_key:
            return routing_key
        raise ValueError("default exchange dead-letter 缺 routing_key，無法辨識目標 queue")
    for dest, bs in bindings_by_dest.items():
        for b in bs:
            if b["source"] == via and b["routing_key"] == (routing_key or ""):
                return dest
    raise ValueError(f"找不到 {via} + rk={routing_key!r} 對應的共用 DLQ")


def main(argv=None):
    import yaml

    ap = argparse.ArgumentParser()
    ap.add_argument("--input", required=True)
    ap.add_argument("--output", required=True)
    args = ap.parse_args(argv)

    with open(args.input, encoding="utf-8") as fh:
        flat = json.load(fh)
    folded = fold(flat)
    with open(args.output, "w", encoding="utf-8") as fh:
        yaml.safe_dump(
            folded, fh, allow_unicode=True, sort_keys=False, default_flow_style=False
        )
    return 0


if __name__ == "__main__":
    sys.exit(main())
