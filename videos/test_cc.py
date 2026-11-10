"""python3 test_cc.py — cc.py's timing of a visible Wait and of the fast parts."""
import cc

tape = """Set TypingSpeed 100ms
# CC: uno
Type "ab"
Enter
# FAST x4
Wait /x/
# WEB: 1
# CC: dos
Sleep 1s
""".splitlines()
log = [(100.0, 100.5, "otro"), (200.0, 208.0, "ab")]  # "ab" ran 8 s after its Enter
fast, webs = [], []
caps, total = cc.captions(tape, webs=webs, fast=fast, log=log)
key = 0.1 + cc.KEY_COST
# the 8 s run, minus nothing typed after Enter: 3 keys + 8 s + 1 s
assert abs(total - (3 * key + 8 + 1)) < 1e-9, total
assert fast == [(3 * key, 8.0, 4)] and webs == [(3 * key + 8, 1)], (fast, webs)
# at 4x the 8 s take 2 s
assert abs(cc.squeeze(total, fast) - (3 * key + 2 + 1)) < 1e-9
assert cc.squeeze(1.0 * key, fast) == key  # before the fast part: unchanged
assert "concat=n=3" in cc.ffmpeg_filter(fast)
assert cc.ffmpeg_filter([(1, 2, 1)]) == ""
# before the recording exists (voice.py fit), the wait counts 0 s
assert abs(cc.captions(tape, strict=False)[1] - (3 * key + 1)) < 1e-9
print("ok")
