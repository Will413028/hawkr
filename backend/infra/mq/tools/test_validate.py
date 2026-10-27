import unittest

from validate import validate


class TestValidate(unittest.TestCase):
    def test_accepts_minimal_valid_topology(self):
        topo = {
            "exchanges": [{"name": "a.b", "type": "topic"}],
            "queues": [{"name": "q", "bindings": [{"exchange": "a.b", "key": "k"}]}],
        }
        self.assertEqual(validate(topo), [])

    def test_rejects_empty_routing_key_from_unquoted_hash(self):
        topo = {
            "exchanges": [{"name": "a.b", "type": "topic"}],
            "queues": [{"name": "q", "bindings": [{"exchange": "a.b", "key": None}]}],
        }
        errs = validate(topo)
        self.assertTrue(any("key" in e for e in errs), errs)

    def test_rejects_retry_and_dlq_together(self):
        topo = {
            "exchanges": [],
            "queues": [{"name": "q", "retry": {"max_attempts": 3}, "dlq": {"via": "x.dlx"}}],
        }
        self.assertTrue(any("互斥" in e for e in validate(topo)))

    def test_rejects_unknown_field(self):
        topo = {"exchanges": [], "queues": [{"name": "q", "retires": {"max_attempts": 3}}]}
        self.assertTrue(any("retires" in e for e in validate(topo)))

    def test_rejects_binding_to_undeclared_exchange(self):
        topo = {
            "exchanges": [{"name": "a.b", "type": "topic"}],
            "queues": [{"name": "q", "bindings": [{"exchange": "typo.name", "key": "k"}]}],
        }
        self.assertTrue(any("typo.name" in e for e in validate(topo)))

    def test_rejects_duplicate_queue_name(self):
        topo = {"exchanges": [], "queues": [{"name": "q"}, {"name": "q"}]}
        self.assertTrue(any("重複" in e for e in validate(topo)))

    def test_accepts_dlq_via_empty_string_as_default_exchange(self):
        topo = {
            "exchanges": [{"name": "hawkr.product.import", "type": "topic"}],
            "queues": [
                {"name": "dlq.product.import.commit"},
                {
                    "name": "product.import.commit",
                    "dlq": {
                        "via": "",
                        "queue": "dlq.product.import.commit",
                        "routing_key": "dlq.product.import.commit",
                    },
                    "bindings": [
                        {"exchange": "hawkr.product.import", "key": "commit"}
                    ],
                },
            ],
        }
        self.assertEqual(validate(topo), [])

    def test_rejects_shared_dlq_missing_via_key_entirely(self):
        topo = {"exchanges": [], "queues": [{"name": "q", "dlq": {"queue": "q.dlq"}}]}
        errs = validate(topo)
        self.assertTrue(any("dlq.via" in e for e in errs), errs)


class TestDeadLetterExchangeExistence(unittest.TestCase):
    def test_rejects_dlq_via_undeclared_exchange(self):
        topo = {
            "exchanges": [{"name": "a.b", "type": "topic"}],
            "queues": [
                {"name": "shared.dlq"},
                {"name": "q", "dlq": {
                    "via": "a.b.dlx", "routing_key": "k", "queue": "shared.dlq",
                }},
            ],
        }
        errs = validate(topo)
        self.assertTrue(any("a.b.dlx" in e and "404" in e for e in errs), errs)

    def test_retry_needs_no_dlx_sibling_on_the_bound_exchange(self):
        topo = {
            "exchanges": [{"name": "payuni.events", "type": "topic"}],
            "queues": [
                {
                    "name": "crm.payuni.thing",
                    "bindings": [{"exchange": "payuni.events", "key": "payuni.thing"}],
                    "retry": {"max_attempts": 3},
                }
            ],
        }
        self.assertEqual(validate(topo), [])

    def test_rejects_shared_dlq_pointing_at_undeclared_queue(self):
        topo = {
            "exchanges": [
                {"name": "a.b", "type": "topic"},
                {"name": "a.b.dlx", "type": "topic"},
            ],
            "queues": [
                {
                    "name": "q",
                    "dlq": {"via": "a.b.dlx", "routing_key": "dlq", "queue": "a.b.dlq"},
                }
            ],
        }
        errs = validate(topo)
        self.assertTrue(any("a.b.dlq" in e for e in errs), errs)

    def test_shared_dlq_target_may_be_declared_later_in_file(self):
        topo = {
            "exchanges": [
                {"name": "a.b", "type": "topic"},
                {"name": "a.b.dlx", "type": "topic"},
            ],
            "queues": [
                {
                    "name": "q",
                    "dlq": {"via": "a.b.dlx", "routing_key": "dlq", "queue": "z.dlq"},
                },
                {"name": "z.dlq", "bindings": [{"exchange": "a.b.dlx", "key": "dlq"}]},
            ],
        }
        self.assertEqual(validate(topo), [])


class TestPolicyUnknownFields(unittest.TestCase):
    def test_rejects_misspelled_dlq_routing_key(self):
        topo = {
            "exchanges": [
                {"name": "aftersales.events.dlx", "type": "topic"},
            ],
            "queues": [
                {"name": "aftersales.events.dlq"},
                {
                    "name": "q",
                    "dlq": {
                        "via": "aftersales.events.dlx",
                        "routing_ky": "dlq",
                        "queue": "aftersales.events.dlq",
                    },
                },
            ],
        }
        errs = validate(topo)
        self.assertTrue(any("routing_ky" in e for e in errs), errs)

    def test_rejects_unknown_retry_field(self):
        topo = {
            "exchanges": [],
            "queues": [{"name": "q", "retry": {"max_attempts": 3, "backoff": "5s"}}],
        }
        errs = validate(topo)
        self.assertTrue(any("backoff" in e for e in errs), errs)


class TestMalformedInputGivesMessageNotTraceback(unittest.TestCase):
    def test_null_retry_is_reported(self):
        topo = {"exchanges": [], "queues": [{"name": "q", "retry": None}]}
        errs = validate(topo)
        self.assertTrue(any("retry" in e and "mapping" in e for e in errs), errs)

    def test_null_dlq_is_reported(self):
        topo = {"exchanges": [], "queues": [{"name": "q", "dlq": None}]}
        errs = validate(topo)
        self.assertTrue(any("dlq" in e and "mapping" in e for e in errs), errs)

    def test_null_bindings_is_reported(self):
        topo = {"exchanges": [], "queues": [{"name": "q", "bindings": None}]}
        errs = validate(topo)
        self.assertTrue(any("bindings" in e and "空值" in e for e in errs), errs)

    def test_absent_bindings_is_fine(self):
        topo = {"exchanges": [], "queues": [{"name": "q"}]}
        self.assertEqual(validate(topo), [])

    def test_scalar_bindings_is_reported(self):
        topo = {"exchanges": [], "queues": [{"name": "q", "bindings": "a.b"}]}
        errs = validate(topo)
        self.assertTrue(any("bindings" in e and "list" in e for e in errs), errs)

    def test_null_root_is_reported(self):
        errs = validate(None)
        self.assertTrue(any("mapping" in e for e in errs), errs)

    def test_scalar_queues_is_reported(self):
        errs = validate({"exchanges": [], "queues": "nope"})
        self.assertTrue(any("queues" in e and "list" in e for e in errs), errs)

    def test_null_queues_is_reported_not_silently_empty(self):
        errs = validate({"exchanges": [], "queues": None})
        self.assertTrue(any("queues" in e and "空值" in e for e in errs), errs)

    def test_null_exchanges_is_reported_not_silently_empty(self):
        errs = validate({"exchanges": None, "queues": []})
        self.assertTrue(any("exchanges" in e and "空值" in e for e in errs), errs)

    def test_absent_top_level_blocks_are_fine(self):
        self.assertEqual(validate({"queues": [{"name": "q"}]}), [])


class TestPrivateDlqContract(unittest.TestCase):
    EXCHANGES = [{"name": "a", "type": "topic"}, {"name": "b", "type": "topic"}]

    def test_accepts_retry_queue_bound_to_two_exchanges(self):
        topo = {
            "exchanges": self.EXCHANGES,
            "queues": [{
                "name": "q",
                "retry": {"max_attempts": 3},
                "bindings": [
                    {"exchange": "a", "key": "k1"},
                    {"exchange": "b", "key": "k2"},
                ],
            }],
        }
        self.assertEqual(validate(topo), [])

    def test_accepts_retry_queue_with_no_bindings(self):
        topo = {
            "exchanges": self.EXCHANGES,
            "queues": [{"name": "q", "retry": {"max_attempts": 3}}],
        }
        self.assertEqual(validate(topo), [])

    def test_rejects_hand_written_dead_letter_args_on_retry_queue(self):
        topo = {
            "exchanges": self.EXCHANGES,
            "queues": [{
                "name": "q",
                "retry": {"max_attempts": 3},
                "bindings": [{"exchange": "a", "key": "k"}],
                "arguments": {"x-dead-letter-exchange": "a.dlx"},
            }],
        }
        errs = validate(topo)
        self.assertTrue(any("x-dead-letter-exchange" in e for e in errs), errs)

    def test_allows_unrelated_arguments(self):
        topo = {
            "exchanges": self.EXCHANGES,
            "queues": [{
                "name": "q",
                "retry": {"max_attempts": 3},
                "bindings": [{"exchange": "a", "key": "k"}],
                "arguments": {"x-max-priority": "5"},
            }],
        }
        self.assertEqual(validate(topo), [])

    def test_rejects_own_dlq_whose_routing_key_is_not_its_own_dlq(self):
        topo = {
            "exchanges": [{"name": "a.dlx", "type": "topic"}],
            "queues": [{"name": "q", "dlq": {"via": "a.dlx", "routing_key": "wrong"}}],
        }
        errs = validate(topo)
        self.assertTrue(any("靜默丟棄" in e for e in errs), errs)

    def test_accepts_own_dlq_with_queue_scoped_routing_key(self):
        topo = {
            "exchanges": [{"name": "a.dlx", "type": "topic"}],
            "queues": [{"name": "q", "dlq": {"via": "a.dlx", "routing_key": "q.dlq"}}],
        }
        self.assertEqual(validate(topo), [])


if __name__ == "__main__":
    unittest.main()
