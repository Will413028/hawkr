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
        {"name": "sample.job.run.retry", "vhost": "/", "durable": True, "auto_delete": False,
         "arguments": {"x-dead-letter-exchange": "hawkr.sample",
                       "x-dead-letter-routing-key": "commit", "x-message-ttl": 10000}},
        {"name": "order.paid.queue", "vhost": "/", "durable": True, "auto_delete": False,
         "arguments": {"x-queue-type": "classic", "x-dead-letter-exchange": "hawkr.order.dlx"}}],
       "bindings": [
        {"source": "hawkr.order", "vhost": "/", "destination": "sample.job.run.retry",
         "destination_type": "queue", "routing_key": "sample.job.run", "arguments": {}}]}
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
        self.assertEqual([e["name"] for e in topo["exchanges"]], ["hawkr.marketing", "hawkr.order"])
        self.assertEqual(len(topo["queues"]), 0)
        self.assertEqual(len(topo["queues_json_args"]), 1)
        self.assertEqual(topo["queues_json_args"][0]["name"], "order.paid.queue")
        self.assertNotIn("x-queue-type", topo["queues_json_args"][0]["arguments"])
        self.assertIn("x-dead-letter-exchange", topo["queues_json_args"][0]["arguments"])
        self.assertEqual(len(topo["bindings"]), 1)
        self.assertIn("prod-only", report)
        self.assertIn("x-message-ttl", report)

    def test_json_args_split(self):
        """queue 的 arguments 含任何非 string 值 → 分流進 queues_json_args，不進 queues；
        binding 指向 queues_json_args 裡的 queue 仍要保留（不可被誤判成「指向被剔除 queue」）。"""
        topo, report = self._run(DEV_JSON_ARGS, EMPTY)
        json_args_names = {q["name"] for q in topo["queues_json_args"]}
        plain_names = {q["name"] for q in topo["queues"]}
        self.assertIn("sample.job.run.retry", json_args_names)
        self.assertNotIn("sample.job.run.retry", plain_names)
        self.assertIn("order.paid.queue", plain_names)
        self.assertNotIn("order.paid.queue", json_args_names)
        self.assertEqual(len(topo["bindings"]), 1)
        self.assertEqual(topo["bindings"][0]["destination"], "sample.job.run.retry")
        self.assertIn("sample.job.run.retry", report)

if __name__ == "__main__":
    unittest.main()
