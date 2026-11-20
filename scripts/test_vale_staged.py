"""Exercise the staged prose lint against a temporary Git repository."""

import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


SCRIPTS = Path(__file__).resolve().parent


def pinned_vale():
    if os.environ.get("VALE"):
        return os.environ["VALE"]
    if shutil.which("mise"):
        result = subprocess.run(
            ["mise", "which", "vale"], cwd=SCRIPTS.parent, capture_output=True, text=True,
        )
        if result.returncode == 0:
            return result.stdout.strip()
    return None


VALE = pinned_vale()


class StagedValeTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.repo = Path(temporary.name)
        self.git("init", "-q")
        self.doc = Path("docs/src/content/docs/Example page.md")
        self.write(self.doc, "backlogerror\n\nBefore.\n")
        self.git("add", "--", self.doc)
        self.commit("baseline")
        self.write(
            "docs/.vale.ini",
            "StylesPath = styles\nMinAlertLevel = suggestion\n\n"
            "[*.{md,mdx}]\nBasedOnStyles = Fixture\n",
        )
        for severity, token in (
            ("error", "backlogerror|stagederror|unstagederror"),
            ("warning", "stagedwarning"),
            ("suggestion", "stagedsuggestion"),
        ):
            self.write(
                f"docs/styles/Fixture/{severity}.yml",
                f'extends: existence\nmessage: "Matched %s"\nlevel: {severity}\n'
                f"scope: text\ntokens:\n  - {token}\n",
            )

    def git(self, *args):
        subprocess.run(["git", *args], cwd=self.repo, check=True)

    def commit(self, message):
        self.git(
            "-c", "user.name=Vale Test", "-c", "user.email=vale@example.test",
            "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgSign=false",
            "commit", "-qm", message,
        )

    def write(self, path, content):
        target = self.repo / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content, encoding="utf-8")

    def check(self, vale=VALE):
        return subprocess.run(
            [sys.executable, SCRIPTS / "vale_staged.py", vale or ""],
            cwd=self.repo,
            capture_output=True,
            text=True,
        )

    def test_no_docs_and_rename_only_skip_vale(self):
        self.write("README.md", "stagederror\n")
        self.git("add", "README.md")
        self.assertEqual(self.check("").returncode, 0)
        self.git("mv", self.doc, self.doc.with_name("Renamed page.md"))
        self.assertEqual(self.check("").returncode, 0)

    @unittest.skipUnless(VALE, "Vale is required")
    def test_only_staged_added_lines_reported_and_advice_passes(self):
        self.write(self.doc, "backlogerror\n\nstagedwarning.\n\nstagedsuggestion.\n")
        self.git("add", "--", self.doc)
        self.write(self.doc, "unstagederror.\n")
        result = self.check()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(f"{self.doc}:3:", result.stdout)
        self.assertIn("warning", result.stdout)
        self.assertIn("suggestion", result.stdout)
        self.assertNotIn("backlogerror", result.stdout)
        self.assertNotIn("unstagederror", result.stdout)

    @unittest.skipUnless(VALE, "Vale is required")
    def test_added_error_fails_even_if_unstaged_fix_is_present(self):
        self.write(self.doc, "backlogerror\n\nstagederror.\n")
        self.git("add", "--", self.doc)
        self.write(self.doc, "Already fixed.\n")
        result = self.check()
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertIn(f"{self.doc}:3:", result.stdout)
        self.assertIn("stagederror", result.stdout)
        self.assertNotIn("backlogerror", result.stdout)

    @unittest.skipUnless(VALE, "Vale is required")
    def test_git_context_settings_do_not_include_unchanged_lines(self):
        self.git("config", "diff.interHunkContext", "5")
        self.write(self.doc, "Old lead.\n\nbacklogerror\n\nOld end.\n")
        self.git("add", "--", self.doc)
        self.commit("before context test")
        self.write(self.doc, "New lead.\n\nbacklogerror\n\nNew end.\n")
        self.git("add", "--", self.doc)
        result = self.check()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn("backlogerror", result.stdout)

    @unittest.skipUnless(VALE, "Vale is required")
    def test_new_mdx_page_fails_on_error(self):
        page = self.doc.with_suffix(".mdx")
        self.write(page, "stagederror.\n")
        self.git("add", "--", page)
        result = self.check()
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertIn(f"{page}:1:", result.stdout)

    def test_staged_docs_with_missing_vale_explain_setup(self):
        self.write(self.doc, "stagederror.\n")
        self.git("add", "--", self.doc)
        result = self.check("")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("make docs-vale-setup", result.stderr)


if __name__ == "__main__":
    unittest.main()
