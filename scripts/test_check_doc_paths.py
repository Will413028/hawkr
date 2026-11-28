import importlib.util
import subprocess
import tempfile
import unittest
from pathlib import Path

_spec = importlib.util.spec_from_file_location(
    "check_doc_paths", Path(__file__).with_name("check-doc-paths.py"))
cdp = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(cdp)

README = """# 文件

## 遺留基準

- `backend/services/`
"""


class CheckDocPathsTest(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.root = Path(self._tmp.name)
        subprocess.run(["git", "init", "-q", str(self.root)], check=True)
        self.write("docs/README.md", README)
        self.write("docs/adr/a.md", "# a\n")
        self.write("backend/docs/plans/b.md", "# b\n")
        self.write("backend/docs/plans/p.md", "# p\n")
        self.write("backend/services/keep.go", "package x\n")

    def tearDown(self):
        self._tmp.cleanup()

    def write(self, rel, text):
        path = self.root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        subprocess.run(["git", "-C", str(self.root), "add", rel], check=True)

    def errors(self):
        return cdp.run(self.root)

    def test_existing_references_pass(self):
        self.write("AGENTS.md", "see `docs/adr/a.md` and backend/docs/plans/p.md#step-1.\n")
        self.assertEqual(self.errors(), [])

    def test_missing_root_reference_fails(self):
        self.write("AGENTS.md", "see `docs/adr/missing.md`\n")
        self.assertEqual(len(self.errors()), 1)

    def test_backend_file_does_not_fall_back_to_backend(self):
        self.write("backend/AGENTS.md", "see `docs/plans/b.md`\n")
        self.assertEqual(len(self.errors()), 1)

    def test_backend_relative_module_spec_fails(self):
        self.write("backend/docs/modules/seller.md", "# seller\n")
        self.write("backend/x.go", "// docs/modules/seller.md\n")
        self.assertEqual(len(self.errors()), 1)

    def test_repo_root_module_spec_passes(self):
        self.write("backend/docs/modules/seller.md", "# seller\n")
        self.write("backend/x.go", "// backend/docs/modules/seller.md\n")
        self.assertEqual(self.errors(), [])

    def test_link_resolving_under_a_doubled_prefix_fails(self):
        self.write("backend/AGENTS.md", "[p](backend/docs/plans/p.md) [q](docs/plans/p.md)\n")
        self.assertEqual(len(self.errors()), 1)

    def test_url_encoded_code_path_is_still_checked(self):
        self.write("AGENTS.md", "`docs/adr/no%20such.md`\n")
        self.assertEqual(len(self.errors()), 1)

    def test_printf_template_is_skipped(self):
        self.write("backend/x.go", '// "backend/docs/modules/%s.md"\n')
        self.assertEqual(self.errors(), [])

    def test_backend_file_missing_everywhere_fails(self):
        self.write("backend/x.go", "// docs/adr/missing.md\n")
        self.assertEqual(len(self.errors()), 1)

    def test_root_file_does_not_fall_back_to_backend(self):
        self.write("website/x.ts", "// docs/plans/b.md\n")
        self.assertEqual(len(self.errors()), 1)

    def test_legacy_files_are_skipped(self):
        self.write("backend/services/x.go", "// docs/adr/missing.md\n")
        self.assertEqual(self.errors(), [])

    def test_header_base_resolves_first(self):
        self.write("docs/adr/c.md", "| 路徑基準 | `backend/` |\n|---|---|\n\n`docs/plans/p.md`\n")
        self.assertEqual(self.errors(), [])

    def test_markdown_relative_link(self):
        self.write("docs/adr/d.md", "[a](a.md) [x](../../backend/docs/plans/p.md)\n")
        self.assertEqual(self.errors(), [])
        self.write("docs/adr/e.md", "[gone](gone.md)\n")
        self.assertEqual(len(self.errors()), 1)

    def test_markdown_root_link_is_url_decoded(self):
        self.write("docs/adr/中.md", "# 中\n")
        self.write("README.md", "[x](/docs/adr/%E4%B8%AD.md) [y](/docs/adr/nope.md)\n")
        self.assertEqual(len(self.errors()), 1)

    def test_unchecked_prefixes_and_placeholders_are_ignored(self):
        self.write("AGENTS.md", "`docs/runbooks/x.md` `docs/modules/{1}.md` `docs/adr/YYYY-MM-DD-<slug>.md` `docs/adr/` `docs/plans/a/rpc-*.csv`\n")
        self.assertEqual(self.errors(), [])

    def test_line_suffixes_are_stripped(self):
        self.write("AGENTS.md", "docs/adr/a.md:L8,L12 docs/adr/a.md:30\n")
        self.assertEqual(self.errors(), [])

    def test_urls_are_ignored(self):
        self.write("AGENTS.md", "https://github.com/o/r/blob/dev/docs/adr/zz.md\n")
        self.assertEqual(self.errors(), [])

    def test_code_span_keeps_spaces_and_fullwidth_parens(self):
        self.write("docs/conventions/api-design-guidelines-connect.md", "# x\n")
        self.write("AGENTS.md", "`docs/conventions/api-design-guidelines-connect.md` `docs/conventions/API 不在（X）.md`\n")
        self.assertEqual(len(self.errors()), 1)

    def test_relative_link_to_docs_is_not_read_from_root(self):
        self.write("docs/adr/中.md", "# 中\n")
        self.write("website/docs/plans/x.md", "# x\n")
        self.write("website/AGENTS.md", "[g](docs/plans/x.md) [e](docs/adr/%E4%B8%AD.md)\n")
        self.assertEqual(self.errors(), ["website/AGENTS.md:1: ](docs/adr/中.md)"])

    def test_bare_template_paths_are_skipped(self):
        self.write("AGENTS.md", "docs/adr/2026-10-30-<slug>.md docs/plans/{step}.md\n")
        self.assertEqual(self.errors(), [])

    def test_untracked_file_does_not_count(self):
        (self.root / "docs/adr/local.md").write_text("x", encoding="utf-8")
        self.write("AGENTS.md", "docs/adr/local.md\n")
        self.assertEqual(len(self.errors()), 1)

    def test_case_must_match(self):
        self.write("AGENTS.md", "docs/adr/A.md\n")
        self.assertEqual(len(self.errors()), 1)

    def test_base_row_only_applies_to_adr_headers(self):
        self.write("backend/docs/plans/q.md", "# q\n")
        self.write("website/n.md", "| 路徑基準 | `backend/` |\n\ndocs/plans/q.md\n")
        self.assertEqual(len(self.errors()), 1)

    def test_readme_must_define_legacy_list(self):
        self.write("docs/README.md", "# 文件\n\n## 其他\n\n- `backend/services/`\n")
        with self.assertRaises(SystemExit):
            self.errors()

    def test_readme_entries_must_be_tracked(self):
        self.write("docs/README.md", README + "- `backend/nowhere/`\n")
        with self.assertRaises(SystemExit):
            self.errors()


if __name__ == "__main__":
    unittest.main()
