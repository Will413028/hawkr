import datetime
import importlib.util
import io
import unittest
import zipfile
from pathlib import Path
import tempfile

_spec = importlib.util.spec_from_file_location("shaded_jar_gate", Path(__file__).with_name("shaded-jar-gate.py"))
gate = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(gate)

NETTY_PROPS = "#Generated\n\nnetty-common.version=4.2.15.Final\nnetty-common.buildDate=x\nnetty-buffer.version=4.2.15.Final\n"
POM = "groupId={g}\nartifactId={a}\nversion={v}\n"


def pom(g, a, v):
    return POM.format(g=g, a=a, v=v)


class BundledInJarTest(unittest.TestCase):
    def test_netty_versions_in_a_shaded_jar(self):
        files = {"META-INF/io.grpc.netty.shaded.io.netty.versions.properties": NETTY_PROPS}
        got = gate.bundled_in_jar("grpc-netty-shaded-1.83.0.jar", files)
        self.assertEqual(
            sorted((c, v) for c, v, _ in got),
            [("io.netty:netty-buffer", "4.2.15.Final"), ("io.netty:netty-common", "4.2.15.Final")],
        )

    def test_own_netty_jar_versions_are_not_bundled(self):
        files = {
            "META-INF/io.netty.versions.properties": NETTY_PROPS,
            "META-INF/maven/io.netty/netty-common/pom.properties": pom("io.netty", "netty-common", "4.1.136.Final"),
            "META-INF/maven/org.jctools/jctools-core/pom.properties": pom("org.jctools", "jctools-core", "4.0.5"),
        }
        got = gate.bundled_in_jar("netty-common-4.1.136.Final.jar", files)
        self.assertEqual([(c, v) for c, v, _ in got], [("org.jctools:jctools-core", "4.0.5")])

    def test_own_pom_is_excluded_longest_artifact_wins(self):
        files = {
            "META-INF/maven/a/lib/pom.properties": pom("a", "lib", "1"),
            "META-INF/maven/a/lib-extra/pom.properties": pom("a", "lib-extra", "2"),
        }
        got = gate.bundled_in_jar("lib-extra-2.jar", files)
        self.assertEqual([(c, v) for c, v, _ in got], [("a:lib", "1")])

    def test_scan_jar_reads_only_relevant_entries(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "outer-1.jar"
            with zipfile.ZipFile(p, "w") as z:
                z.writestr("META-INF/maven/x/outer/pom.properties", pom("x", "outer", "1"))
                z.writestr("META-INF/maven/y/inner/pom.properties", pom("y", "inner", "9"))
                z.writestr("a/B.class", b"")
            self.assertEqual([(c, v) for c, v, _ in gate.scan_jar(p)], [("y:inner", "9")])


class EvaluateTest(unittest.TestCase):
    TODAY = datetime.date(2026, 11, 15)
    FINDINGS = {("io.netty:netty-common", "4.1.1"): [{"id": "GHSA-1", "aliases": ["CVE-1"]}]}

    def test_unlisted_fails(self):
        failed, allowed = gate.evaluate(self.FINDINGS, {}, self.TODAY)
        self.assertEqual(len(failed), 1)
        self.assertEqual(allowed, [])

    def test_listed_by_id_or_alias_is_allowed(self):
        for ident in ("GHSA-1", "CVE-1"):
            failed, allowed = gate.evaluate(self.FINDINGS, {ident: datetime.date(2027, 1, 1)}, self.TODAY)
            self.assertEqual(failed, [])
            self.assertEqual(len(allowed), 1)

    def test_expired_ignore_fails(self):
        failed, _ = gate.evaluate(self.FINDINGS, {"GHSA-1": datetime.date(2026, 11, 14)}, self.TODAY)
        self.assertEqual(len(failed), 1)
        self.assertIn("已過", failed[0])

    def test_load_ignores(self):
        toml = '[[IgnoredVulns]]\nid = "GHSA-1"\nignoreUntil = 2027-02-13\nreason = "r"\n'
        self.assertEqual(gate.load_ignores(toml), {"GHSA-1": datetime.date(2027, 2, 13)})


if __name__ == "__main__":
    unittest.main()
