"""Check repository-local Markdown links without network access."""
import re
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit

root = Path(__file__).resolve().parents[1]
documents = [root / name for name in ("README.md", "CONTRIBUTING.md", "SECURITY.md")]
documents += sorted((root / "docs").rglob("*.md"))
failures = []
for document in documents:
    text = document.read_text(encoding="utf-8-sig")
    text = re.sub(r"```.*?```", "", text, flags=re.S)
    for match in re.finditer(r"\]\((<[^>]+>|[^\s)]+)(?:\s+[^)]*)?\)", text):
        target = match.group(1).strip("<>")
        parsed = urlsplit(target)
        if parsed.scheme or parsed.netloc or not parsed.path:
            continue
        path = (document.parent / unquote(parsed.path)).resolve()
        if not path.exists():
            failures.append("{}: {}".format(document.relative_to(root), target))
if failures:
    print("Broken documentation links:\n" + "\n".join(failures), file=sys.stderr)
    sys.exit(1)
print("Checked local links in {} Markdown documents".format(len(documents)))
