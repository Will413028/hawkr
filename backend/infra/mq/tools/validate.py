
import argparse
import sys

QUEUE_FIELDS = {
    "name", "bindings", "retry", "dlq", "arguments", "args_json",
    "go_const", "java_const", "description",
}
EXCHANGE_FIELDS = {"name", "type", "go_const", "java_const", "description"}
BINDING_FIELDS = {"exchange", "key"}
DLQ_FIELDS = {"via", "routing_key", "queue"}
RETRY_FIELDS = {"max_attempts"}
VALID_TYPES = {"topic", "direct", "fanout", "headers"}


def _as_list(container, key, errors):
    if key not in container:
        return []
    value = container[key]
    if value is None:
        errors.append(
            f"{key} 是空值——`{key}:` 後面沒接東西會 parse 成 None；"
            f"真的要表達空清單就寫 `{key}: []`"
        )
        return []
    if not isinstance(value, list):
        errors.append(
            f"{key} 必須是 list，實際是 {type(value).__name__}（{value!r}）"
        )
        return []
    return value


def validate(topo):
    errors = []
    if not isinstance(topo, dict):
        return [
            f"topology 根層必須是 mapping，實際是 {type(topo).__name__}（{topo!r}）"
            f"——空檔或縮排整段跑掉時會長這樣"
        ]
    exchange_names = set()

    for i, e in enumerate(_as_list(topo, "exchanges", errors)):
        where = f"exchanges[{i}]"
        if not isinstance(e, dict):
            errors.append(
                f"{where}: 必須是 mapping，實際是 {type(e).__name__}（{e!r}）"
            )
            continue
        unknown = set(e) - EXCHANGE_FIELDS
        if unknown:
            errors.append(f"{where}: 未知欄位 {sorted(unknown)}")
        if not e.get("name"):
            errors.append(f"{where}: name 必填且非空")
            continue
        if e["name"] in exchange_names:
            errors.append(f"{where}: exchange 名稱重複 {e['name']}")
        exchange_names.add(e["name"])
        if e.get("type") not in VALID_TYPES:
            errors.append(f"{where} ({e['name']}): type 必須是 {sorted(VALID_TYPES)}")

    queue_names = set()
    pending_shared_dlq = []
    for i, q in enumerate(_as_list(topo, "queues", errors)):
        where = f"queues[{i}]"
        if not isinstance(q, dict):
            errors.append(
                f"{where}: 必須是 mapping，實際是 {type(q).__name__}（{q!r}）"
            )
            continue
        unknown = set(q) - QUEUE_FIELDS
        if unknown:
            errors.append(f"{where}: 未知欄位 {sorted(unknown)}")
        name = q.get("name")
        if not name:
            errors.append(f"{where}: name 必填且非空")
            continue
        where = f"{where} ({name})"
        if name in queue_names:
            errors.append(f"{where}: queue 名稱重複")
        queue_names.add(name)

        if "retry" in q and "dlq" in q:
            errors.append(f"{where}: retry 與 dlq 互斥")

        retry = q.get("retry")
        if "retry" in q and not isinstance(retry, dict):
            errors.append(
                f"{where}: retry 必須是 mapping，實際是 {type(retry).__name__}"
                f"（{retry!r}）——`retry:` 後面沒接東西會 parse 成 None，"
                f"正確寫法是 `retry: {{max_attempts: 3}}`"
            )
            retry = None
        dlq = q.get("dlq")
        if "dlq" in q and not isinstance(dlq, dict):
            errors.append(
                f"{where}: dlq 必須是 mapping，實際是 {type(dlq).__name__}"
                f"（{dlq!r}）——`dlq:` 後面沒接東西會 parse 成 None，"
                f"正確寫法是 `dlq: {{via: <exchange>.dlx}}`"
            )
            dlq = None
        if "bindings" not in q:
            bindings = []
        elif q["bindings"] is None:
            errors.append(
                f"{where}: bindings 是空值——`bindings:` 後面沒接東西會 parse 成 None，"
                f"會讓 expand.py 拋 TypeError、讓 terraform 拋 "
                f'"Iteration over null value"；沒有 binding 就把整行拿掉'
            )
            bindings = []
        elif isinstance(q["bindings"], list):
            bindings = q["bindings"]
        else:
            errors.append(
                f"{where}: bindings 必須是 list，實際是 "
                f"{type(q['bindings']).__name__}（{q['bindings']!r}）"
            )
            bindings = []

        if isinstance(retry, dict):
            unknown = set(retry) - RETRY_FIELDS
            if unknown:
                errors.append(
                    f"{where}: retry 未知欄位 {sorted(unknown)}"
                    f"（合法欄位 {sorted(RETRY_FIELDS)}）——拼錯的欄位會被靜默忽略"
                )
            attempts = retry.get("max_attempts")
            if not isinstance(attempts, int) or attempts < 1:
                errors.append(f"{where}: retry.max_attempts 必須是正整數")

        if isinstance(dlq, dict):
            unknown = set(dlq) - DLQ_FIELDS
            if unknown:
                errors.append(
                    f"{where}: dlq 未知欄位 {sorted(unknown)}"
                    f"（合法欄位 {sorted(DLQ_FIELDS)}）——拼錯的欄位會被靜默忽略："
                    f"例如 routing_key 打成 routing_ky 時 x-dead-letter-routing-key 不會被"
                    f"注入，死信改用原 routing key 進 DLX、沒有 binding 命中 → 靜默丟棄"
                )
            if "via" not in dlq:
                errors.append(
                    f"{where}: dlq.via 必填（空字串代表 RabbitMQ default exchange，是合法值）"
                )
            else:
                via = dlq["via"]
                if via != "" and via not in exchange_names:
                    errors.append(
                        f"{where}: dlq.via {via!r} 不是已宣告的 exchange"
                        f"——terraform plan 仍會乾淨產出，apply 會在建 dead-letter binding 時"
                        f"撞 404 not_found 並留下 partial apply。請在 exchanges: 補宣告它，"
                        f"或改用既有的 DLX（空字串 '' = default exchange 才可免宣告）"
                    )
            if "queue" in dlq:
                pending_shared_dlq.append((where, dlq["queue"]))
            elif dlq.get("routing_key") != f"{name}.dlq":
                errors.append(
                    f"{where}: dlq 沒有 queue: 時死信只會被 {dlq.get('via')!r} 上 routing key "
                    f"{name}.dlq 的 binding 接住，所以 dlq.routing_key 必須是 {name!r}.dlq"
                    f"（目前 {dlq.get('routing_key')!r}）——對不上就是沒有 binding 命中、"
                    f"死信靜默丟棄"
                )

        if "retry" in q:
            hand_written = sorted(
                k for k in (q.get("arguments") or {})
                if isinstance(k, str) and k.startswith("x-dead-letter-")
            )
            if hand_written:
                errors.append(
                    f"{where}: retry: queue 不得手寫 {hand_written}"
                    f"——x-dead-letter-exchange 固定是 {name}.dlx（自動產生的 fanout），"
                    f"由展開注入；fanout 不看 routing key，所以沒有 x-dead-letter-routing-key"
                )

        for j, b in enumerate(bindings):
            if not isinstance(b, dict):
                errors.append(
                    f"{where}.bindings[{j}]: 必須是 mapping，"
                    f"實際是 {type(b).__name__}（{b!r}）"
                )
                continue
            bw = f"{where}.bindings[{j}]"
            unknown = set(b) - BINDING_FIELDS
            if unknown:
                errors.append(f"{bw}: 未知欄位 {sorted(unknown)}")
            key = b.get("key")
            if not isinstance(key, str) or key == "":
                errors.append(
                    f"{bw}: key 必須是非空字串（`#` 與 `*` 在 YAML 需加引號，"
                    f"漏引號會 parse 成 None）"
                )
            if b.get("exchange") not in exchange_names:
                errors.append(f"{bw}: exchange {b.get('exchange')!r} 未宣告")

    for where, shared in pending_shared_dlq:
        if shared not in queue_names:
            errors.append(
                f"{where}: dlq.queue {shared!r} 不是已宣告的 queue 條目"
                f"——共用 DLQ 不由展開規則生成，必須自己有一條 top-level queue 條目"
                f"（具名 DLX 的還要自帶 DLX→自己的 binding）；缺了它死信會被投進 DLX "
                f"但沒有 queue 承接 → 靜默丟棄"
            )

    return errors


def main(argv=None):
    import yaml

    ap = argparse.ArgumentParser()
    ap.add_argument("--topology", required=True)
    args = ap.parse_args(argv)

    with open(args.topology, encoding="utf-8") as fh:
        topo = yaml.safe_load(fh)

    errors = validate(topo)
    for e in errors:
        print(f"ERROR {e}", file=sys.stderr)
    if errors:
        print(f"\n{len(errors)} 個問題", file=sys.stderr)
        return 1
    print("topology.yaml OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
