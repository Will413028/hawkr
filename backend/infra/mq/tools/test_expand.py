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
        # 🔴 這一段是 merge 衝突的解，兩邊的數字**都不對**（2026-09-12）。
        #    dev 那批把 exchanges 推到 122（+31），我這批把 queues／bindings 各 +2
        #    （66 §W2-2 的 shipping.uber.webhook ＋ 展開出的 .dlq）。
        #    git 對這種「各自改同一個 pin」只會選一邊，而兩邊各自都是對自己那半的正確值
        #    ⇒ 真值必須**從合併後的 topology.yaml 實算**，不得挑一邊。
        #    實算結果：exchanges 122 / queues 189 / queues_json_args 6 / bindings 243。
        #    增減量對帳（不是為了讓測試變綠改數字）：
        #      共同祖先 91 / 187 / 6 / 241 = 525
        #      dev  側   +31 /  +0 / +0 /  +0   （procurement 等）
        #      本  側    +0 /  +2 / +0 /  +2   （shipping.uber.webhook：dlq: 分支不產生
        #                                       exchange，只有 retry: 會產 .retry fanout）
        #      合計      122 / 189 /  6 / 243 = 560 ✓
        #
        # 以下保留兩邊原本的逐項對帳註解，它們各自仍然成立，只是都只涵蓋自己那一半。
        #
        # ── dev 側原註解 ──

        # ── 本側原註解 ──
        # 2026-08-30 #2904：新增 invoice.order.cancelled（帶 retry: max_attempts=3，
        # 一條 binding）；2026-09-04 membership billing 新增 buyer/seller result 與
        # recurring webhook 三組 queue；2026-09-05 procurement quote-invite 新增
        # procurement.events／.dlx 與 notification.procurement。逐項對帳，新增資源全部
        # 有歸屬、沒有一個是「多出來的」：
        # exchanges        87 → 90  (+3)  procurement.events ＋ .dlx ＋
        # notification.procurement.retry（fanout）
        # queues          183 → 185 (+2)  notification.procurement ＋ .dlq
        # queues_json_args  6 →   6 (+0)  本條目沒有 args_json
        # bindings        235 → 238 (+3) procurement.events→queue（key=#）＋
        # retry fanout→queue ＋ procurement.events.dlx→.dlq
        # 2026-09-12 66 號 W2-2：新增 shipping.uber.webhook（帶 dlq: via shipping.webhook.dlx，
        # 一條 binding payuni.events/uber.shipping.status）。逐項對帳：
        # exchanges        91 →  91 (+0)  沿用既有 payuni.events 與 shipping.webhook.dlx，
        # dlq: 分支不產生 exchange（只有 retry: 會產 .retry fanout）
        # queues          187 → 189 (+2)  shipping.uber.webhook ＋ 展開出的 shipping.uber.webhook.dlq
        # queues_json_args  6 →   6 (+0)  本條目沒有 args_json
        # bindings        241 → 243 (+2)  payuni.events→queue（key=uber.shipping.status）＋
        # shipping.webhook.dlx→.dlq（key=queue 名）
        # 2026-09-17 Batch A（N16 責任判定 financially final）：新增**兩條** queue——
        # order.fulfillment.responsibility_finalized（dlq: via hawkr.order.dlx）＋
        # payment.fulfillment.responsibility_finalized（dlq: via payment.events.dlx）。
        # 一條 routing key 兩條 queue 是刻意的：order 與 payment 對同一件事做兩件不同的
        # 副作用（訂單終態轉換 vs receivable 處置），合併會讓一邊的失敗拖垮另一邊。
        # 逐項對帳（🔴 先算增減量再改數字，不得只為了讓測試變綠而改）：
        # exchanges       122 → 122 (+0)  沿用既有 hawkr.shipping / hawkr.order.dlx /
        #                                 payment.events.dlx；兩條都走 dlq: 分支，
        #                                 而 dlq: 不產生 exchange（只有 retry: 會產 .retry fanout）
        # queues          189 → 193 (+4)  兩條 queue ＋ 各自展開出的 .dlq
        #                                 （只給 via + routing_key、不帶 queue: ＝ 自帶 DLQ 形狀）
        # queues_json_args  6 →   6 (+0)  兩個條目都沒有 args_json
        # bindings        243 → 247 (+4)  每條各 2：hawkr.shipping→queue
        #                                 （key=fulfillment.responsibility.finalized）
        #                                 ＋ 各自 DLX→.dlq（key=queue 名）
        #
        # Batch A-R1（logistics.cost.incurred，70 §3.1.4 durable join 的成本側入口）：
        # exchanges       122 → 122 (+0)  沿用既有 hawkr.shipping（來源）與
        #                                 payment.events.dlx（DLX 跟著 consumer 服務走，
        #                                 不是跟著來源 exchange，36 §6.2 註）
        # queues          193 → 195 (+2)  payment.logistics.cost_incurred ＋ 展開出的 .dlq
        # queues_json_args  6 →   6 (+0)  無 args_json
        # bindings        247 → 249 (+2)  hawkr.shipping→queue（key=logistics.cost.incurred）
        #                                 ＋ payment.events.dlx→.dlq（key=queue 名）
        #
        # buyer-service 刪除：buyer.account.deleted、buyer.order.completed、buyer.product.restock
        # 沒有消費者（單體的 buyer consumer 自己宣告 queue），三條各帶 retry:
        # exchanges       122 → 116 (-6)  每條的 .retry ＋ .dlx（fanout）
        # queues          195 → 189 (-6)  每條的 queue ＋ .dlq
        # queues_json_args  6 →   6 (+0)
        # bindings        249 → 240 (-9)  每條的來源 binding ＋ .retry→queue ＋ .dlx→.dlq
        #
        # social.account.deleted 刪除（social 的討論區、留言、讚都改由 buyer.deleted 與
        # seller.deleted 清理）：帶 retry:
        # 這次之前的 HEAD 實算是 116 / 187 / 6 / 238：queues 與 bindings 的 pin（189、240）
        # 早已各差 2，這裡一併校正成實算值。
        # exchanges       116 → 114 (-2)  .retry ＋ .dlx（fanout）
        # queues          187 → 185 (-2)  queue ＋ .dlq
        # queues_json_args  6 →   6 (+0)
        # bindings        238 → 235 (-3)  來源 binding ＋ .retry→queue ＋ .dlx→.dlq
        #
        # 刪除沒有 producer 或 consumer 已不存在的 8 條帶 retry: 的 queue（ai_service.order.completed、
        # ai_service.seller.membership_changed、seller.account.deleted、seller.product.viewed、
        # seller.review.created、seller.review.hidden、seller.review.result，以及已刪除服務的 seller.deleted 消費 queue）：
        # exchanges       114 →  98 (-16)  每條的 .retry ＋ .dlx（fanout）
        # queues          185 → 169 (-16)  每條的 queue ＋ .dlq
        # queues_json_args  6 →   6 (+0)
        # bindings        235 → 211 (-24)  每條的來源 binding ＋ .retry→queue ＋ .dlx→.dlq
        #
        # 刪除沒有消費者的會員付款流程的三條 queue（buyer.membership.payment.result、
        # seller.membership.payment.result、payment.payuni.membership.recurring），三條都走 dlq: 分支：
        # exchanges        98 →  98 (+0)  沿用既有 payment.events.dlx 與 payment.webhook.dlx，
        #                                 dlq: 不產生 exchange
        # queues          169 → 163 (-6)  每條的 queue ＋ 展開出的 .dlq
        # queues_json_args  6 →   6 (+0)  三個條目都沒有 args_json
        # bindings        211 → 203 (-8)  buyer 與 seller 各 2 條來源 binding
        #                                 （payment.membership.{buyer,seller}.{success,failed}）＋ 各 1 條
        #                                 payment.events.dlx→.dlq；recurring 1 條來源 binding
        #                                 （payuni.events→payuni.payment.recurring）＋ 1 條
        #                                 payment.webhook.dlx→.dlq
        self.assertEqual(len(out["exchanges"]), 98, "exchanges 數量與 pin 不符")
        self.assertEqual(len(out["queues"]), 163, "queues 數量與 pin 不符")
        self.assertEqual(
            len(out["queues_json_args"]), 6, "queues_json_args 數量與 pin 不符"
        )
        self.assertEqual(len(out["bindings"]), 203, "bindings 數量與 pin 不符")
        # 98 + 163 + 6 + 203 = 470
        self.assertEqual(sum(len(v) for v in out.values()), 470)


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
                    "name": "product.import.commit.retry",
                    "args_json": True,
                    "arguments": {
                        "x-dead-letter-exchange": "hawkr.product.import",
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
