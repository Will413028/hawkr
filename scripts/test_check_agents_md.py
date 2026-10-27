import importlib.util
import pathlib
import unittest

spec = importlib.util.spec_from_file_location(
    "check_agents_md", pathlib.Path(__file__).with_name("check-agents-md.py")
)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)


class LimitTest(unittest.TestCase):
    def test_tiers(self):
        self.assertEqual(mod.limit_for("AGENTS.md"), mod.ROOT_LIMIT)
        self.assertEqual(mod.limit_for("website/AGENTS.md"), mod.SUBPROJECT_LIMIT)
        self.assertEqual(mod.limit_for("backend/internal/modules/seller/AGENTS.md"), mod.NESTED_LIMIT)
        self.assertEqual(mod.limit_for("packages/hawkr_auth/AGENTS.md"), mod.NESTED_LIMIT)

    def test_size_is_counted_in_bytes(self):
        # 中文一字 3 bytes：2,001 字就超過 6,000 bytes。
        text = "\n".join(["字" * 400] * 5) + "字"
        self.assertTrue(mod.violations("packages/x/AGENTS.md", text))
        self.assertFalse(mod.violations("website/AGENTS.md", text))

    def test_line_length_is_counted_in_characters(self):
        self.assertFalse(mod.violations("AGENTS.md", "字" * mod.LINE_LIMIT))
        self.assertTrue(mod.violations("AGENTS.md", "字" * (mod.LINE_LIMIT + 1)))

    def test_nextjs_block_is_not_counted(self):
        block = (
            "<!-- BEGIN:nextjs-agent-rules -->\n"
            + "x" * 20_000
            + "\n<!-- END:nextjs-agent-rules -->\n"
        )
        self.assertFalse(mod.violations("website/AGENTS.md", "# website\n" + block))


class CheckTest(unittest.TestCase):
    big = "x\n" * 7_000

    def test_oversize_file_fails(self):
        self.assertTrue(mod.check({"packages/x/AGENTS.md": self.big}, {}))

    def test_listed_exception_passes(self):
        self.assertFalse(mod.check({"packages/x/AGENTS.md": self.big}, {"packages/x/AGENTS.md": "步驟 4"}))

    def test_exception_that_no_longer_exceeds_fails(self):
        self.assertTrue(mod.check({"packages/x/AGENTS.md": "short"}, {"packages/x/AGENTS.md": "步驟 4"}))

    def test_exception_for_missing_file_fails(self):
        self.assertTrue(mod.check({}, {"packages/gone/AGENTS.md": "步驟 4"}))


if __name__ == "__main__":
    unittest.main()
