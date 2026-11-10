# Videos

Tutorial episodes recorded with [VHS](https://github.com/charmbracelet/vhs): every keystroke and pause lives in a `.tape`, so a video is reproducible and has no typos. Spanish, with closed captions.

| Episode | Tape | Starts from |
| --- | --- | --- |
| 0 — Qué es versaDeploy (intro animada, Remotion) | `motion/script.json` | `motion/render.sh` |
| 0b — Instalar versaDeploy (piloto) | `episodes/00-instalar.tape` | `./prep.sh 00` |
| 2 — Laravel: de un VPS vacío a la app en línea | `episodes/02-laravel.tape` | `./prep.sh 02` |
| 3 — FastAPI + PostgreSQL: .env, servicio y un deploy incremental | `episodes/03-fastapi.tape` | `./prep.sh 03` |
| 4 — Nuxt con SSR: .env en tiempo de ejecución y un rollback | `episodes/04-nuxt.tape` | `./prep.sh 04` |
| 5 — versaWYS en FrankenPHP: del composer create-project al panel en línea | `episodes/05-versawys.tape` | `./prep.sh 05` |
| 6 — Cuando un deploy falla: los errores más comunes | `episodes/06-problemas.tape` | after `./make.sh 05` (continues its server) |
| 7 — De git push a producción: GitHub Actions y versaDeploy en tu propio VPS | `episodes/07-github-actions.tape` | `./prep.sh 07` (demo repo `kriollo/versadeploy-demo-ci` + the runner in `ci-runner/`) |

## The intro (Remotion)

`motion/` is a [Remotion](https://www.remotion.dev) project (React → video): an animated explainer of what versaDeploy is and does, with the same voice. `motion/script.json` holds the scenes and their narration; `motion/prepare.py` synthesizes every line and makes each scene last exactly as long as its lines (no drift by construction), and takes screenshots from the Laravel episode. `npx remotion studio` (in `motion/`) previews it live.

```bash
cd motion && npm ci && ./render.sh   # → out/00-intro-voice.mp4, -voice-cc.mp4, 00-intro.srt
```

Remotion is free for individuals and companies of up to 3 people; bigger companies need a [company license](https://www.remotion.dev/license).

## Render

```bash
./make.sh 02     # everything → out/02-laravel-final.mp4 (+ 02-laravel-full.srt) and -final-cc.mp4
```

The final episode is built in Remotion (`motion/`) around the terminal recording: an animated opening with the steps, a card that explains each chapter before it is shown (from `episodes/NN-name.json`; chapters are the tape's `# ─── N. Title ───` markers), the recording itself, the deployed app in a browser window (`shot.sh`, taken while the server still runs it; `shot-login.mjs` when the json has `browser.login`) and an outro. `motion/episode.sh NN` rebuilds it from an existing recording.

or step by step:

```bash
./prep.sh 02                                  # fresh "VPS" (Docker) + the episode's project
python3 voice.py fit episodes/02-laravel.tape # lengthen pauses where a spoken line doesn't fit
./render.sh episodes/02-laravel.tape          # video + captions
python3 voice.py mix episodes/02-laravel.tape # voice-over → out/02-laravel-voice*.mp4
```

`out/` gets:

- `NN-name.mp4` — 1920×1080, with an empty band at the bottom for captions. Upload this one to YouTube together with
- `NN-name.srt` — the closed captions (YouTube: *Subtitles → Upload file → With timing*);
- `NN-name-cc.mp4` — the same video with the captions burned in, for networks without CC (Instagram, LinkedIn, X);
- `NN-name-voice.mp4` / `NN-name-voice-cc.mp4` — the same two with the voice-over reading the captions. No background music: YouTube's Content ID claimed the track we used.

## Voice-over

- **Voice**: [Kokoro-82M](https://huggingface.co/hexgrad/Kokoro-82M) (Apache-2.0), local, on the GPU when there is one, voice `em_alex`. One-time setup (Python 3.12): `python3.12 -m venv .work/tts2 && .work/tts2/bin/pip install kokoro soundfile`; `kokoro_tts.py` synthesizes every line of a video in one batch. Also available: `TTS=piper` (offline, CPU; `.work/tts` + a model in `.work/voices`) and `TTS=elevenlabs` (paid API; key in `.work/elevenlabs_api_key.txt`, voice id in `.work/elevenlabs_voice.txt`).
- **Pronunciation**: `pronunciation.tsv` tells the Spanish voice how to read English words (`deploy` → `diplói`). Add a line when a word sounds wrong; the captions keep the original text.
- **Timing**: each line starts with its caption. `voice.py fit` synthesizes every line and adds a `Sleep` after the caption where the spoken line is longer than the caption stays on screen, so lines never overlap (lines are cached in `.work/voice-cache` by engine, voice and text).

## Publishing

`youtube.py` writes `out/<video>-youtube.txt`: a YouTube description with clickable chapters taken from the final timeline. For the docs site, the final videos and their captions (`.vtt`) are assets of the `media` pre-release; `site/videos_page.py` rebuilds the site's Videos page from the descriptions.

## How it works

- **The "VPS"** is `server.sh up`: an Ubuntu 24.04 container with systemd and sshd (the e2e image), SSH on `127.0.0.1:2299`, HTTP on `127.0.0.1:8099` (`http://<app>.localhost:8099` in any browser).
- **Sandbox**: each tape first sources `setup.sh`, which points `HOME` to `.work/home` (its own SSH key, `known_hosts` and `~/.local/bin/versa`), so a recording never touches your real home.
- **Captions** are `# CC: text` comments in the tape. `cc.py` adds up what is visible (typing at `TypingSpeed`, keys, `Sleep`) to time each one; anything of variable length (a deploy, an install) goes between `Hide` and `Show`, so the video cuts from its start to its end and the timing stays exact.
- **Fast-forward**: a visible `Wait` (no `Hide`) stays in the video, timed from the command log `setup.sh` writes (when each command started and ended, `out/NN-name.cmdlog`); `# FAST x4` before it plays it 4 times faster with a "▶▶ x4" badge (`render.sh`). Episode 7's deploys use it. Check: `python3 test_cc.py`.
- **Web shots**: `web N URL` (hidden, between `Hide` and `Show`) takes the page as a browser shows it at that moment; `# WEB: N` is where `motion/episode.py` puts it, as a browser card with the voice of `webs.N.lines` in the episode's json.
- **Waiting**: `Wait+Screen` only inside the TUI (it redraws the whole screen). After a command whose output scrolls, VHS no longer sees the new screen, so those waits match the cursor line instead: `Wait /question\?/` or the prompt `Wait /❯\s*$/`.
