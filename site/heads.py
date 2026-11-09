#!/usr/bin/env python3
"""Lists a doc's headings with the anchor MkDocs gives them: the Spanish translation
keeps the same anchor ({ #anchor }), so links work in both languages."""
import html, re, sys

from markdown.extensions.toc import slugify

for f in sys.argv[1:]:
    seen, code = {}, False
    for line in open(f, encoding="utf-8"):
        if line.startswith("```"):
            code = not code
        m = None if code else re.match(r"^(#{1,6}) (.+)$", line.rstrip())
        if not m:
            continue
        # inline only: "6. Title" is a heading, not a list item
        text = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", m[2]).replace("`", "")
        s = slugify(html.unescape(text), "-")
        n = seen.get(s, 0)
        seen[s] = n + 1
        print(f"{m[1]} {m[2]}  ->  #{s}_{n}" if n else f"{m[1]} {m[2]}  ->  #{s}")
