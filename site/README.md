# Docs site

[versadeploy.websystem.cl](https://versadeploy.websystem.cl): MkDocs Material, Spanish at `/` and English at `/en/`, published by [`.github/workflows/site.yml`](../.github/workflows/site.yml) on every push that touches `site/`, `doc/` or `examples/`.

- `docs/*.es.md` / `*.en.md`: the guides (home, videos, getting started, flows, examples, comparison).
- The reference comes from [`../doc`](../doc) (English, the single source); Spanish translations go in `translations/<NAME>.es.md`. `build.py` assembles both into `docs/reference/`.
- Videos: assets of the `media` pre-release (`.mp4` + `.vtt`), downloaded at build time. `videos_page.py` rebuilds the Videos page from `../videos/out/*-youtube.txt`.

```bash
python3 -m venv .venv && .venv/bin/pip install mkdocs-material mkdocs-static-i18n
.venv/bin/python build.py serve      # live preview at http://127.0.0.1:8000
```
