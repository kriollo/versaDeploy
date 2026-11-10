import React from "react";
import { AbsoluteFill, Audio, Img, OffthreadVideo, Sequence, interpolate, staticFile, useCurrentFrame } from "remotion";
import { Background, C, Chip, Fade, MONO, Rise, SANS, Title, center, useIn } from "./Intro";

type Line = { text: string; start: number; dur: number; file: string };
type Seg = {
  type: "opening" | "video" | "chapter" | "browser" | "outro";
  start: number;
  dur: number;
  lines: Line[];
  from?: number;
  title?: string;
  subtitle?: string;
  steps?: string[];
  num?: number;
  bullets?: string[];
  visual?: string;
  url?: string;
  shot?: string;
  next?: string;
};
export type EpisodeTimeline = { fps: number; total: number; video: string; segments: Seg[] };

// ── cards ───────────────────────────────────────────────────────────────────

const Opening: React.FC<{ s: Seg }> = ({ s }) => {
  const p = useIn(0, 12);
  return (
    <AbsoluteFill style={{ ...center, flexDirection: "column" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 28, opacity: p, transform: `scale(${0.8 + 0.2 * p})` }}>
        <Img src={staticFile("logo.webp")} style={{ width: 120, borderRadius: 26 }} />
        <div style={{ fontFamily: SANS, fontSize: 52, fontWeight: 800, color: C.muted }}>versaDeploy</div>
      </div>
      <Rise delay={12}>
        <div style={{ fontFamily: SANS, fontSize: 110, fontWeight: 900, marginTop: 30, background: `linear-gradient(90deg, ${C.blue}, ${C.purple})`, WebkitBackgroundClip: "text", color: "transparent", textAlign: "center" }}>
          {s.title}
        </div>
      </Rise>
      <Rise delay={24}>
        <div style={{ fontFamily: SANS, fontSize: 44, color: C.text, marginTop: 6 }}>{s.subtitle}</div>
      </Rise>
      <div style={{ display: "flex", gap: 22, marginTop: 70 }}>
        {s.steps!.map((t, i) => (
          <Rise key={t} delay={50 + i * 12}>
            <div style={{ width: 300, background: C.panel, borderRadius: 18, padding: "22px 24px", borderTop: `6px solid ${[C.cyan, C.orange, C.purple, C.green, C.blue][i % 5]}` }}>
              <div style={{ fontFamily: MONO, fontSize: 30, color: C.muted }}>{i + 1}</div>
              <div style={{ fontFamily: SANS, fontSize: 30, fontWeight: 700, color: C.text, marginTop: 6 }}>{t}</div>
            </div>
          </Rise>
        ))}
      </div>
    </AbsoluteFill>
  );
};

const EnvDiagram: React.FC = () => {
  const frame = useCurrentFrame();
  const box = (label: string, sub: string, color: string, delay: number) => (
    <Rise delay={delay}>
      <div style={{ width: 560, background: C.panel, border: `3px solid ${color}`, borderRadius: 18, padding: "22px 30px" }}>
        <div style={{ fontFamily: MONO, fontSize: 34, color }}>{label}</div>
        <div style={{ fontFamily: SANS, fontSize: 26, color: C.text, marginTop: 6 }}>{sub}</div>
      </div>
    </Rise>
  );
  const dash = interpolate(frame, [40, 70], [0, 1], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 60 }}>
      <div style={{ display: "flex", flexDirection: "column", gap: 26 }}>
        {box("releases/v1/app/.env", "enlace →", C.muted, 10)}
        {box("releases/v2/app/.env", "enlace →", C.muted, 18)}
        {box("current/app/.env", "la versión en línea →", C.blue, 26)}
      </div>
      <div style={{ fontSize: 80, color: C.green, opacity: dash }}>⟶</div>
      {box("shared/.env", "el archivo real: uno por servidor, fuera de git", C.green, 45)}
    </div>
  );
};

const RollbackDiagram: React.FC = () => {
  const frame = useCurrentFrame();
  const y = interpolate(frame, [70, 90], [1, 0], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
  const rows = ["releases/v1", "releases/v2"];
  return (
    <div style={{ position: "relative", width: 1100, height: 260 }}>
      {rows.map((r, i) => (
        <Rise key={r} delay={10 + i * 10} style={{ position: "absolute", left: 0, top: i * 130 }}>
          <div style={{ width: 600, background: C.panel, borderRadius: 18, padding: "24px 32px", fontFamily: MONO, fontSize: 40, color: C.text, border: `3px solid ${Math.round(y) === i ? C.green : C.panel}` }}>{r}</div>
        </Rise>
      ))}
      <div style={{ position: "absolute", left: 660, top: 26 + y * 130, fontFamily: MONO, fontSize: 44, color: C.blue }}>← current</div>
    </div>
  );
};

const Chapter: React.FC<{ s: Seg }> = ({ s }) => {
  const p = useIn(0, 12);
  const palette = [C.green, C.blue, C.purple, C.orange, C.cyan];
  return (
    <AbsoluteFill>
      <div style={{ position: "absolute", top: 110, left: 0, right: 0, ...center, gap: 34, opacity: p }}>
        <div style={{ width: 110, height: 110, borderRadius: 55, ...center, fontFamily: SANS, fontSize: 60, fontWeight: 900, color: C.bg, background: `linear-gradient(135deg, ${C.blue}, ${C.purple})`, transform: `scale(${0.5 + 0.5 * p})` }}>{s.num}</div>
        <div style={{ fontFamily: SANS, fontSize: 76, fontWeight: 800, color: C.text }}>{s.title}</div>
      </div>
      <div style={{ position: "absolute", top: 330, left: 0, right: 0, bottom: 160, ...center }}>
        {s.visual === "env" ? (
          <EnvDiagram />
        ) : s.visual === "rollback" ? (
          <RollbackDiagram />
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
            {s.bullets!.map((t, i) => (
              <Rise key={t} delay={20 + i * 10}>
                <div style={{ fontFamily: SANS, fontSize: 46, color: C.text, display: "flex", gap: 24, alignItems: "center" }}>
                  <span style={{ width: 18, height: 18, borderRadius: 9, background: palette[i % palette.length] }} />
                  {t}
                </div>
              </Rise>
            ))}
          </div>
        )}
      </div>
    </AbsoluteFill>
  );
};

const Browser: React.FC<{ s: Seg }> = ({ s }) => {
  const frame = useCurrentFrame();
  const p = useIn(0, 16);
  const zoom = interpolate(frame, [0, 300], [1, 1.04], { extrapolateRight: "clamp" });
  return (
    <AbsoluteFill style={{ ...center, paddingBottom: 90 }}>
      <div style={{ width: 1500, borderRadius: 18, overflow: "hidden", background: "#fff", boxShadow: "0 40px 100px #000a", opacity: p, transform: `translateY(${(1 - p) * 60}px)` }}>
        <div style={{ height: 64, background: "#dee1e6", display: "flex", alignItems: "center", gap: 14, padding: "0 20px" }}>
          {[C.red, C.orange, C.green].map((c) => (
            <div key={c} style={{ width: 16, height: 16, borderRadius: 8, background: c }} />
          ))}
          <div style={{ marginLeft: 18, flex: 1, height: 40, borderRadius: 20, background: "#fff", display: "flex", alignItems: "center", padding: "0 22px", fontFamily: SANS, fontSize: 24, color: "#3c4043" }}>
            <span style={{ color: "#5f6368", marginRight: 12 }}>ⓘ</span>
            {s.url}
          </div>
        </div>
        <div style={{ overflow: "hidden", height: 843 }}>
          <Img src={staticFile(s.shot!)} style={{ width: "100%", display: "block", transform: `scale(${zoom})`, transformOrigin: "50% 0%" }} />
        </div>
      </div>
    </AbsoluteFill>
  );
};

const Outro: React.FC<{ s: Seg }> = ({ s }) => {
  const p = useIn(0, 12);
  return (
    <AbsoluteFill style={{ ...center, flexDirection: "column" }}>
      <Img src={staticFile("logo.webp")} style={{ width: 260, borderRadius: 36, boxShadow: "0 20px 60px #0008", opacity: p, transform: `scale(${0.6 + 0.4 * p})` }} />
      <div style={{ fontFamily: SANS, fontSize: 100, fontWeight: 900, background: `linear-gradient(90deg, ${C.blue}, ${C.purple})`, WebkitBackgroundClip: "text", color: "transparent", opacity: p }}>versaDeploy</div>
      <Rise delay={20}>
        <div style={{ fontFamily: MONO, fontSize: 42, color: C.cyan }}>github.com/kriollo/versaDeploy</div>
      </Rise>
      {s.next ? (
        <Rise delay={45} style={{ marginTop: 60, display: "flex", alignItems: "center", gap: 24 }}>
          <div style={{ fontFamily: SANS, fontSize: 38, color: C.muted }}>Próximo episodio</div>
          <Chip text={s.next} delay={50} color={C.green} />
        </Rise>
      ) : null}
    </AbsoluteFill>
  );
};

// ── composition ─────────────────────────────────────────────────────────────

const Clip: React.FC<{ s: Seg; video: string; fps: number }> = ({ s, video, fps }) => (
  <AbsoluteFill style={{ background: C.bg2 }}>
    <OffthreadVideo src={staticFile(video)} startFrom={Math.round(s.from! * fps)} muted />
  </AbsoluteFill>
);

export const Episode: React.FC<{ ep: string; timeline: EpisodeTimeline | null; captions: boolean }> = ({ timeline: t, captions }) => {
  const frame = useCurrentFrame();
  if (!t) return null;
  const fps = t.fps;
  const all = t.segments.flatMap((s) => s.lines.map((l) => ({ a: s.start + l.start, b: s.start + l.start + l.dur, text: l.text })));
  const sec = frame / fps;
  const cap = all.find((l) => sec >= l.a && sec < l.b + 0.3);
  return (
    <AbsoluteFill>
      <Background />
      {t.segments.map((s, i) => {
        const dur = Math.round(s.dur * fps);
        const body =
          s.type === "video" ? <Clip s={s} video={t.video} fps={fps} /> :
          s.type === "opening" ? <Opening s={s} /> :
          s.type === "chapter" ? <Chapter s={s} /> :
          s.type === "browser" ? <Browser s={s} /> : <Outro s={s} />;
        return (
          <Sequence key={i} from={Math.round(s.start * fps)} durationInFrames={dur}>
            {s.type === "video" ? body : <Fade dur={dur}>{body}</Fade>}
            {s.lines.map((l) => (
              <Sequence key={l.file} from={Math.round(l.start * fps)} layout="none">
                <Audio src={staticFile(l.file)} />
              </Sequence>
            ))}
          </Sequence>
        );
      })}
      {captions && cap && (
        <div style={{ position: "absolute", bottom: 34, left: 0, right: 0, ...center }}>
          <div style={{ maxWidth: 1500, textAlign: "center", fontFamily: SANS, fontSize: 38, color: "#fff", background: "#000b", padding: "12px 26px", borderRadius: 10, lineHeight: 1.35 }}>{cap.text}</div>
        </div>
      )}
    </AbsoluteFill>
  );
};
