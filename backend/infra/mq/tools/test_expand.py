import json
import pathlib
import unittest

from expand import ExpandError, expand


class TestRetryExpansion(unittest.TestCase):
    def test_retry_expands_to_five_resources(self):
        topo = {
            "exchanges": [{"name": "hawkr.account", "type": "topic"}],
            "queues": [
                {
                    "name": "im.account.deleted",
                    "bindings": [
                        {"exchange": "hawkr.account", "key": "account.deleted"}
                    ],
                    "retry": {"max_attempts": 3},
                }
            ],
        }
        out = expand(topo)
        base = _fanout("im.account.deleted.retry")
        dlx = _fanout("im.account.deleted.dlx")

        self.assertIn(base, out["exchanges"])
        self.assertIn(_bind("im.account.deleted.retry", "im.account.deleted", ""), out["bindings"])
        self.assertIn(dlx, out["exchanges"])
        self.assertIn(
            {"name": "im.account.deleted.dlq", "durable": True,
             "auto_delete": False, "arguments": {}},
            out["queues"],
        )
        self.assertIn(_bind("im.account.deleted.dlx", "im.account.deleted.dlq", ""), out["bindings"])

        base_q = [q for q in out["queues"] if q["name"] == "im.account.deleted"][0]
        self.assertEqual(
            base_q["arguments"],
            {"x-dead-letter-exchange": "im.account.deleted.dlx"},
        )

    def test_dlx_does_not_depend_on_the_bound_exchange(self):
        topo = {
            "exchanges": [
                {"name": "a", "type": "topic"},
                {"name": "b", "type": "topic"},
            ],
            "queues": [
                {
                    "name": "q",
                    "bindings": [
                        {"exchange": "a", "key": "k1"},
                        {"exchange": "a", "key": "k2"},
                        {"exchange": "b", "key": "k3"},
                    ],
                    "retry": {"max_attempts": 3},
                }
            ],
        }
        out = expand(topo)
        dlq_bindings = [b for b in out["bindings"] if b["destination"] == "q.dlq"]
        self.assertEqual(dlq_bindings, [_bind("q.dlx", "q.dlq", "")])
        base = [q for q in out["queues"] if q["name"] == "q"][0]
        self.assertEqual(base["arguments"], {"x-dead-letter-exchange": "q.dlx"})


class TestDlqExpansion(unittest.TestCase):
    def test_own_dlq_uses_the_named_dlx_with_a_queue_scoped_routing_key(self):
        topo = {
            "queues": [
                {
                    "name": "order.payment.result",
                    "dlq": {
                        "via": "hawkr.payment.dlx",
                        "routing_key": "order.payment.result.dlq",
                    },
                }
            ]
        }
        out = expand(topo)
        self.assertIn("order.payment.result.dlq", [q["name"] for q in out["queues"]])
        self.assertIn(
            _bind("hawkr.payment.dlx", "order.payment.result.dlq",
                  "order.payment.result.dlq"),
            out["bindings"],
        )
        base = [q for q in out["queues"] if q["name"] == "order.payment.result"][0]
        self.assertEqual(
            base["arguments"],
            {
                "x-dead-letter-exchange": "hawkr.payment.dlx",
                "x-dead-letter-routing-key": "order.payment.result.dlq",
            },
        )

    def test_shared_dlq_emits_no_own_queue_and_keeps_named_dlx(self):
        topo = {
            "queues": [
                {
                    "name": "aftersales.shipping.label",
                    "dlq": {
                        "via": "aftersales.events.dlx",
                        "routing_key": "dlq",
                        "queue": "aftersales.events.dlq",
                    },
                }
            ]
        }
        out = expand(topo)
        base = [q for q in out["queues"] if q["name"] == "aftersales.shipping.label"][0]
        self.assertEqual(
            base["arguments"],
            {
                "x-dead-letter-exchange": "aftersales.events.dlx",
                "x-dead-letter-routing-key": "dlq",
            },
        )
        self.assertEqual([q["name"] for q in out["queues"]], ["aftersales.shipping.label"])
        self.assertEqual(out["exchanges"], [])
        self.assertEqual(out["bindings"], [])


def _fanout(name):
    return {"name": name, "type": "fanout", "durable": True,
            "auto_delete": False, "arguments": {}}


def _bind(source, destination, routing_key):
    return {"source": source, "destination": destination,
            "destination_type": "queue", "routing_key": routing_key, "arguments": {}}


_TOPOLOGY_YAML = pathlib.Path(__file__).parent.parent / "topology" / "topology.yaml"
_TOPOLOGY_JSON = pathlib.Path(__file__).parent.parent / "topology" / "topology.json"


class TestExpandedCountsAgainstRealTopology(unittest.TestCase):
    def test_expanded_resource_counts_are_pinned(self):
        import yaml

        with _TOPOLOGY_YAML.open(encoding="utf-8") as fh:
            topo = yaml.safe_load(fh)
        out = expand(topo)
        # 94 + 147 + 0 + 193 = 434
        self.assertEqual(len(out["exchanges"]), 94, "exchanges 數量與 pin 不符")
        self.assertEqual(len(out["queues"]), 147, "queues 數量與 pin 不符")
        self.assertEqual(
            len(out["queues_json_args"]), 0, "queues_json_args 數量與 pin 不符"
        )
        self.assertEqual(len(out["bindings"]), 193, "bindings 數量與 pin 不符")
        self.assertEqual(sum(len(v) for v in out.values()), 434)


@unittest.skipUnless(
    _TOPOLOGY_JSON.exists(),
    "topology.json 已在 P1 CI 接線後刪除（2026-07-26 git rm）；此測試的任務（證明摺疊"
    "等價）已完成。災難復原重跑：git show <刪除前 commit>:infra/mq/topology/"
    "topology.json > infra/mq/topology/topology.json 還原檔案後再跑此測試。",
)
class TestRoundTripAgainstRealTopology(unittest.TestCase):
    def _canonical(self, flat):
        return {
            "exchanges": sorted(
                (json.dumps(x, sort_keys=True) for x in flat["exchanges"])
            ),
            "queues": sorted((json.dumps(x, sort_keys=True) for x in flat["queues"])),
            "queues_json_args": sorted(
                (json.dumps(x, sort_keys=True) for x in flat["queues_json_args"])
            ),
            "bindings": sorted(
                (json.dumps(x, sort_keys=True) for x in flat["bindings"])
            ),
        }

    def test_round_trip_is_identity(self):
        from fold import fold

        here = pathlib.Path(__file__).parent
        original = json.loads(
            (here.parent / "topology" / "topology.json").read_text(encoding="utf-8")
        )
        rebuilt = expand(fold(original))

        a = self._canonical(original)
        b = self._canonical(rebuilt)
        for key in ("exchanges", "queues", "queues_json_args", "bindings"):
            with self.subTest(kind=key):
                only_original = set(a[key]) - set(b[key])
                only_rebuilt = set(b[key]) - set(a[key])
                self.assertEqual(only_original, set(), f"{key}: 摺疊後遺失")
                self.assertEqual(only_rebuilt, set(), f"{key}: 摺疊後多出")
                self.assertEqual(len(a[key]), len(b[key]), f"{key}: 數量不符")

    def test_expected_resource_counts(self):
        from fold import fold

        here = pathlib.Path(__file__).parent
        original = json.loads(
            (here.parent / "topology" / "topology.json").read_text(encoding="utf-8")
        )
        folded = fold(original)
        self.assertEqual(len(folded["exchanges"]), 42)
        self.assertEqual(len([q for q in folded["queues"] if not q.get("args_json")]), 91)
        self.assertEqual(len([q for q in folded["queues"] if q.get("args_json")]), 6)
        self.assertEqual(
            sum(len(q.get("bindings", [])) for q in folded["queues"]), 100
        )


class TestArgsJsonAndErrors(unittest.TestCase):
    def test_args_json_goes_to_separate_list(self):
        topo = {
            "queues": [
                {
                    "name": "sample.job.run.retry",
                    "args_json": True,
                    "arguments": {
                        "x-dead-letter-exchange": "hawkr.sample",
                        "x-dead-letter-routing-key": "commit",
                        "x-message-ttl": 10000,
                    },
                }
            ]
        }
        out = expand(topo)
        self.assertEqual(out["queues"], [])
        self.assertEqual(len(out["queues_json_args"]), 1)
        self.assertEqual(out["queues_json_args"][0]["arguments"]["x-message-ttl"], 10000)

    def test_bare_queue_expands_to_itself(self):
        topo = {"queues": [{"name": "payment.rescue.refund.dlq", "bindings": [
            {"exchange": "hawkr.payment.dlx", "key": "payment.rescue.refund.dlq"}
        ]}]}
        out = expand(topo)
        self.assertEqual(len(out["queues"]), 1)
        self.assertEqual(len(out["bindings"]), 1)

    def test_retry_and_dlq_are_mutually_exclusive(self):
        topo = {"queues": [{"name": "x", "retry": {"max_attempts": 3}, "dlq": {"via": "y.dlx"}}]}
        with self.assertRaises(ExpandError):
            expand(topo)

    def test_bad_max_attempts_rejected(self):
        topo = {"queues": [{"name": "x", "retry": {"max_attempts": 0}}]}
        with self.assertRaises(ExpandError):
            expand(topo)


class TestDLQIsolationInvariants(unittest.TestCase):
    def _topo(self):
        import yaml

        with _TOPOLOGY_YAML.open(encoding="utf-8") as fh:
            return yaml.safe_load(fh)

    def _private_dlq_queues(self, topo):
        return [q for q in topo["queues"] if "retry" in q]

    def test_every_private_dlq_has_its_own_fanout_dlx(self):
        topo = self._topo()
        out = expand(topo)
        kinds = {e["name"]: e["type"] for e in out["exchanges"]}
        broken = []
        for q in self._private_dlq_queues(topo):
            dlx = f"{q['name']}.dlx"
            if kinds.get(dlx) != "fanout":
                broken.append((q["name"], f"{dlx} type={kinds.get(dlx)!r}，應為 'fanout'"))
        self.assertEqual(broken, [], f"{len(broken)} 條 queue 沒有專屬 fanout DLX")

    def test_private_dlx_is_bound_to_exactly_its_own_dlq(self):
        topo = self._topo()
        out = expand(topo)
        by_source = {}
        for b in out["bindings"]:
            by_source.setdefault(b["source"], []).append(b)
        broken = []
        for q in self._private_dlq_queues(topo):
            dlx, dlq = f"{q['name']}.dlx", f"{q['name']}.dlq"
            bs = by_source.get(dlx, [])
            if len(bs) != 1:
                broken.append((q["name"], f"{dlx} 有 {len(bs)} 條出口，應為 1"))
            elif bs[0]["destination"] != dlq:
                broken.append((q["name"], f"{dlx} 指向 {bs[0]['destination']}，應為 {dlq}"))
            elif bs[0]["routing_key"] != "":
                broken.append((q["name"], f"fanout binding 帶了 routing key {bs[0]['routing_key']!r}"))
        self.assertEqual(broken, [], f"{len(broken)} 條 DLX 的出口不是自己的 DLQ")

    def test_private_dlq_queues_carry_dlx_and_no_routing_key(self):
        topo = self._topo()
        out = expand(topo)
        args = {q["name"]: q["arguments"] for q in out["queues"] + out["queues_json_args"]}
        broken = []
        for q in self._private_dlq_queues(topo):
            a = args[q["name"]]
            if a.get("x-dead-letter-exchange") != f"{q['name']}.dlx":
                broken.append((q["name"], f"dlx={a.get('x-dead-letter-exchange')!r}"))
            elif "x-dead-letter-routing-key" in a:
                broken.append((q["name"], "fanout DLX 不該有 x-dead-letter-routing-key"))
        self.assertEqual(broken, [], f"{len(broken)} 條 queue 的 dead-letter args 不對")

    def test_no_dlq_binding_uses_a_wildcard_routing_key(self):
        out = expand(self._topo())
        bad = [
            (b["source"], b["destination"], b["routing_key"])
            for b in out["bindings"]
            if b["destination"].endswith(".dlq")
            and ("#" in b["routing_key"].split(".") or "*" in b["routing_key"].split("."))
        ]
        self.assertEqual(bad, [], f"{len(bad)} 條 DLQ binding 用 wildcard routing key")

    def test_no_two_dlqs_share_a_dlx_routing_key(self):
        seen = {}
        for b in expand(self._topo())["bindings"]:
            if b["destination"].endswith(".dlq"):
                seen.setdefault((b["source"], b["routing_key"]), []).append(b["destination"])
        shared = {k: sorted(v) for k, v in seen.items() if len(v) > 1}
        self.assertEqual(shared, {}, f"{len(shared)} 組 (DLX, key) 被多個 DLQ 共用")


if __name__ == "__main__":
    unittest.main()
