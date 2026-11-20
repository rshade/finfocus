"""Report Vale findings on added lines in the staged documentation."""

import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


DOCS = "docs/src/content/docs"
HUNK = re.compile(rb"^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@", re.MULTILINE)


def git(*args):
    return subprocess.check_output(["git", *args])


def added_lines(old_blob, new_blob):
    patch = git(
        "diff", "--unified=0", "--inter-hunk-context=0", "--no-color",
        "--no-ext-diff", "--no-textconv", "--text", old_blob, new_blob, "--",
    )
    for hunk in HUNK.finditer(patch):
        start = int(hunk[1])
        yield range(start, start + int(hunk[2] or b"1"))


def staged_pages():
    # Include paths outside the docs tree so Git can detect cross-directory renames.
    diff = git("diff", "--cached", "--raw", "-z", "--no-abbrev", "--find-renames", "--")
    entries = iter(diff.split(b"\0")[:-1])
    for header in entries:
        old_mode, new_mode, old_blob, new_blob, status = header.split()
        path = os.fsdecode(next(entries))
        if status[:1] in (b"R", b"C"):
            path = os.fsdecode(next(entries))
        if not (
            new_mode.startswith(b"100")
            and path.startswith(DOCS + "/")
            and path.endswith((".md", ".mdx"))
        ):
            continue

        # A new regular file has no previous text. For an edited or renamed
        # file, compare its blobs so a rename alone contributes zero lines.
        ranges = None
        if old_mode.startswith(b":100"):
            ranges = tuple(added_lines(old_blob, new_blob))
            if not any(ranges):
                continue
        yield path, new_blob, ranges


def lint_pages(vale, pages):
    config = Path("docs/.vale.ini").resolve()
    with tempfile.TemporaryDirectory(prefix="finfocus-vale-") as snapshot:
        for path, (blob, _) in pages.items():
            copy = Path(snapshot, path)
            copy.parent.mkdir(parents=True, exist_ok=True)
            copy.write_bytes(git("cat-file", "blob", blob))
        run = subprocess.run(
            [vale, f"--config={config}", "--no-global", "--output=JSON", "--no-exit", DOCS],
            cwd=snapshot,
            stdout=subprocess.PIPE,
            check=True,
        )
        return json.loads(run.stdout)


def main(vale):
    pages = {path: (blob, ranges) for path, blob, ranges in staged_pages()}
    if not pages:
        return 0
    if not vale:
        print("Vale is unavailable. Install it with mise, then run make docs-vale-setup.", file=sys.stderr)
        return 2

    # Vale checks complete pages for markup context. Filter its findings to
    # the ranges Git reports on the new side of each hunk, as CI does.
    has_errors = False
    for path, findings in sorted(lint_pages(vale, pages).items()):
        path = Path(path).as_posix()
        _, ranges = pages[path]
        for finding in findings:
            line = finding["Line"]
            if ranges is not None and not any(line in added for added in ranges):
                continue
            severity = finding["Severity"]
            column = finding["Span"][0]
            print(f"{path}:{line}:{column}: {severity}: {finding['Message']} ({finding['Check']})")
            has_errors |= severity == "error"
    return int(has_errors)


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv[1]))
    except (OSError, subprocess.CalledProcessError, ValueError) as error:
        print(f"Staged Vale check failed ({error}). Run make docs-vale-setup.", file=sys.stderr)
        sys.exit(2)
