
import argparse
import json
import sys


class ExpandError(Exception):
    pass


def _exchange(name, type_):
    return {
        "name": name,
        "type": type_,
        "durable": True,
        "auto_delete": False,
        "arguments": {},
    }


def _queue(name, arguments=None):
    return {
        "name": name,
        "durable": True,
        "auto_delete": False,
        "arguments": arguments if arguments is not None else {},
    }


def _binding(source, destination, routing_key):
    return {
        "source": source,
        "destination": destination,
        "destination_type": "queue",
        "routing_key": routing_key,
        "arguments": {},
    }


def expand(topo):
    exchanges = [_exchange(e["name"], e["type"]) for e in topo.get("exchanges", [])]
    queues = []
    queues_json_args = []
    bindings = []

    for q in topo.get("queues", []):
        name = q["name"]
        if "retry" in q and "dlq" in q:
            raise ExpandError(f"{name}: retry 與 dlq 互斥，不能同時宣告")

        args = dict(q.get("arguments", {}))
        own_bindings = q.get("bindings", [])
        for b in own_bindings:
            bindings.append(_binding(b["exchange"], name, b["key"]))

        if "retry" in q:
            attempts = q["retry"].get("max_attempts")
            if not isinstance(attempts, int) or attempts < 1:
                raise ExpandError(f"{name}: retry.max_attempts 必須是正整數")
            retry_ex = f"{name}.retry"
            dlx = f"{name}.dlx"
            dlq = f"{name}.dlq"
            exchanges.append(_exchange(retry_ex, "fanout"))
            exchanges.append(_exchange(dlx, "fanout"))
            bindings.append(_binding(retry_ex, name, ""))
            queues.append(_queue(dlq))
            bindings.append(_binding(dlx, dlq, ""))
            args["x-dead-letter-exchange"] = dlx

        elif "dlq" in q:
            d = q["dlq"]
            via = d["via"]
            args["x-dead-letter-exchange"] = via
            if "routing_key" in d:
                args["x-dead-letter-routing-key"] = d["routing_key"]
            if "queue" not in d:
                dlq = f"{name}.dlq"
                queues.append(_queue(dlq))
                bindings.append(_binding(via, dlq, dlq))

        entry = _queue(name, args)
        if q.get("args_json"):
            queues_json_args.append(entry)
        else:
            queues.append(entry)

    return {
        "exchanges": exchanges,
        "queues": queues,
        "queues_json_args": queues_json_args,
        "bindings": bindings,
    }


def _load(path):
    import yaml

    with open(path, encoding="utf-8") as fh:
        return yaml.safe_load(fh)


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("--topology", required=True)
    ap.add_argument("--emit-expanded", required=True)
    args = ap.parse_args(argv)

    out = expand(_load(args.topology))
    with open(args.emit_expanded, "w", encoding="utf-8") as fh:
        json.dump(out, fh, ensure_ascii=False, indent=2, sort_keys=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
