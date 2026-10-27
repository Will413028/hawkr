import json, subprocess, sys, tempfile, unittest, pathlib

DEV = {"exchanges": [
        {"name": "hawkr.order", "vhost": "/", "type": "topic", "durable": True, "auto_delete": False, "arguments": {}},
        {"name": "amq.topic", "vhost": "/", "type": "topic", "durable": True, "auto_delete": False, "arguments": {}}],
       "queues": [
        {"name": "order.paid.queue", "vhost": "/", "durable": True, "auto_delete": False,
         "arguments": {"x-queue-type": "classic", "x-dead-letter-exchange": "hawkr.order.dlx",
                       "x-message-ttl": 60000}},
        {"name": "gateway.sse.pod-abc", "vhost": "/", "durable": False, "auto_delete": True, "arguments": {}}],
       "bindings": [
        {"source": "hawkr.order", "vhost": "/", "destination": "order.paid.queue",
         "destination_type": "queue", "routing_key": "order.paid", "arguments": {}},
        {"source": "hawkr.order", "vhost": "/", "destination": "gateway.sse.pod-abc",
         "destination_type": "queue", "routing_key": "order.paid", "arguments": {}}]}
DEV_JSON_ARGS = {"exchanges": [
        {"name": "hawkr.order", "vhost": "/", "type": "topic", "durable": True, "auto_delete": False, "arguments": {}}],
       "queues": [
        {"name": "product.import.commit.retry", "vhost": "/", "durable": True, "auto_delete": False,
         "arguments": {"x-dead-letter-exchange": "hawkr.product.import",
                       "x-dead-letter-routing-key": "commit", "x-message-ttl": 10000}},
        {"name": "order.paid.queue", "vhost": "/", "durable": True, "auto_delete": False,
         "arguments": {"x-queue-type": "classic", "x-dead-letter-exchange": "hawkr.order.dlx"}}],
       "bindings": [
        {"source": "hawkr.order", "vhost": "/", "destination": "product.import.commit.retry",
         "destination_type": "queue", "routing_key": "product.import.commit", "arguments": {}}]}
EMPTY = {"exchanges": [], "queues": [], "bindings": []}
PROD = {"exchanges": [
        {"name": "hawkr.order", "vhost": "/", "type": "topic", "durable": True, "auto_delete": False, "arguments": {}},
        {"name": "hawkr.marketing", "vhost": "/", "type": "topic", "durable": True, "auto_delete": False, "arguments": {}}],
        "queues": [], "bindings": []}

class TestGen(unittest.TestCase):
    def _run(self, dev, prod):
        d = pathlib.Path(tempfile.mkdtemp())
        (d / "dev.json").write_text(json.dumps(dev))
        (d / "prod.json").write_text(json.dumps(prod))
        subprocess.run([sys.executable, str(pathlib.Path(__file__).parent / "definitions_to_topology.py"),
                        "--dev", str(d / "dev.json"), "--prod", str(d / "prod.json"),
                        "--out", str(d / "topo.json"), "--report", str(d / "report.md")], check=True)
        topo = json.loads((d / "topo.json").read_text())
        report = (d / "report.md").read_text()
        return topo, report

    def test_generate(self):
        topo, report = self._run(DEV, PROD)
        self.assertEqual([e["name"] for e in topo["exchanges"]], ["hawkr.marketing", "hawkr.order"])  # amq.* 剔除、union、排序
        # order.paid.queue 帶非 string arg（x-message-ttl: 60000）→ 分流進 queues_json_args，不進 queues
        self.assertEqual(len(topo["queues"]), 0)                                     # gateway.sse.* 剔除 + 唯一剩下的 queue 因 int arg 分流走
        self.assertEqual(len(topo["queues_json_args"]), 1)
        self.assertEqual(topo["queues_json_args"][0]["name"], "order.paid.queue")
        self.assertNotIn("x-queue-type", topo["queues_json_args"][0]["arguments"])        # x-queue-type 剝除
        self.assertIn("x-dead-letter-exchange", topo["queues_json_args"][0]["arguments"]) # 其餘 args 保留
        self.assertEqual(len(topo["bindings"]), 1)                              # 指向被剔除 queue 的 binding 一併剔除；指向 json_args queue 的 binding 保留
        self.assertIn("prod-only", report)
        self.assertIn("x-message-ttl", report)  # 非 string arg 要被 audit 點名（module 用 map(string) 送出會變型別）

    def test_json_args_split(self):
        """queue 的 arguments 含任何非 string 值 → 分流進 queues_json_args，不進 queues；
        binding 指向 queues_json_args 裡的 queue 仍要保留（不可被誤判成「指向被剔除 queue」）。"""
        topo, report = self._run(DEV_JSON_ARGS, EMPTY)
        json_args_names = {q["name"] for q in topo["queues_json_args"]}
        plain_names = {q["name"] for q in topo["queues"]}
        self.assertIn("product.import.commit.retry", json_args_names)   # x-message-ttl 是 int → json_args
        self.assertNotIn("product.import.commit.retry", plain_names)
        self.assertIn("order.paid.queue", plain_names)                  # 剩下皆 string arg（x-queue-type 已被剝除）→ 留在 queues
        self.assertNotIn("order.paid.queue", json_args_names)
        # binding 指向 json_args queue，不應被當成「指向被剔除 queue」而濾掉
        self.assertEqual(len(topo["bindings"]), 1)
        self.assertEqual(topo["bindings"][0]["destination"], "product.import.commit.retry")
        self.assertIn("product.import.commit.retry", report)  # audit report 仍點名該 queue 的非 string arg

if __name__ == "__main__":
    unittest.main()
