import React from "react";
import {
  AbsoluteFill,
  Audio,
  Img,
  Sequence,
  interpolate,
  spring,
  staticFile,
  useCurrentFrame,
  useVideoConfig,
} from "remotion";

// Tokyo Night, like versa's TUI
export const C = {
  bg: "#1a1b26",
  bg2: "#16161e",
  panel: "#24283b",
  text: "#c0caf5",
  muted: "#565f89",
  blue: "#7aa2f7",
  cyan: "#7dcfff",
  purple: "#bb9af7",
  green: "#9ece6a",
  orange: "#e0af68",
  red: "#f7768e",
};
export const SANS = "'Noto Sans', 'Inter', system-ui, sans-serif";
export const MONO = "'JetBrainsMono Nerd Font', 'JetBrains Mono', monospace";

export type Line = { text: string; start: number; dur: number; file: string };
export type Scene = {
  id: string;
  start: number;
  dur: number;
  lines: Line[];
  title?: string;
  items?: string[];
  steps?: { icon: string; name: string; desc: string }[];
  answer?: string;
  shot?: string;
  code?: string;
  url?: string;
  next?: string[];
};
export type Timeline = { fps: number; total: number; scenes: Scene[] };

// ── building blocks ─────────────────────────────────────────────────────────

export const useIn = (delay: number, damping = 14) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  return spring({ frame: frame - delay, fps, config: { damping } });
};

export const Rise: React.FC<{ delay?: number; children: React.ReactNode; style?: React.CSSProperties }> = ({ delay = 0, children, style }) => {
  const p = useIn(delay);
  return <div style={{ opacity: p, transform: `translateY(${(1 - p) * 40}px)`, ...style }}>{children}</div>;
};

export const Title: React.FC<{ text: string }> = ({ text }) => (
  <Rise style={{ position: "absolute", top: 90, left: 0, right: 0, textAlign: "center" }}>
    <div style={{ fontFamily: SANS, fontSize: 76, fontWeight: 800, color: C.text, letterSpacing: -1 }}>{text}</div>
    <div style={{ margin: "22px auto 0", width: 120, height: 6, borderRadius: 3, background: `linear-gradient(90deg, ${C.blue}, ${C.purple})` }} />
  </Rise>
);

export const Chip: React.FC<{ text: string; delay: number; color?: string; size?: number }> = ({ text, delay, color = C.blue, size = 38 }) => {
  const p = useIn(delay, 12);
  return (
    <div
      style={{
        opacity: p,
        transform: `scale(${0.6 + 0.4 * p})`,
        fontFamily: SANS,
        fontSize: size,
        fontWeight: 600,
        color: C.text,
        background: C.panel,
        border: `2px solid ${color}55`,
        borderLeft: `8px solid ${color}`,
        borderRadius: 16,
        padding: "18px 30px",
      }}
    >
      {text}
    </div>
  );
};

export const Window: React.FC<{ src: string; delay?: number; width: number }> = ({ src, delay = 0, width }) => {
  const frame = useCurrentFrame();
  const p = useIn(delay, 16);
  const zoom = interpolate(frame, [delay, delay + 400], [1, 1.06], { extrapolateRight: "clamp" });
  return (
    <div
      style={{
        width,
        opacity: p,
        transform: `translateY(${(1 - p) * 60}px)`,
        borderRadius: 18,
        overflow: "hidden",
        background: C.bg2,
        boxShadow: "0 30px 80px #0009",
        border: `2px solid ${C.panel}`,
      }}
    >
      <div style={{ height: 34, display: "flex", gap: 10, alignItems: "center", paddingLeft: 16, background: C.panel }}>
        {[C.red, C.orange, C.green].map((c) => (
          <div key={c} style={{ width: 14, height: 14, borderRadius: 7, background: c }} />
        ))}
      </div>
      <div style={{ overflow: "hidden" }}>
        <Img src={staticFile(src)} style={{ width: "100%", display: "block", transform: `scale(${zoom})`, transformOrigin: "30% 30%" }} />
      </div>
    </div>
  );
};

export const Typing: React.FC<{ text: string; delay: number; cps?: number; prompt?: string }> = ({ text, delay, cps = 28, prompt = "$" }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const n = Math.max(0, Math.floor(((frame - delay) / fps) * cps));
  const cursor = Math.floor(frame / 15) % 2 === 0;
  return (
    <span style={{ fontFamily: MONO }}>
      <span style={{ color: C.purple }}>{prompt} </span>
      <span style={{ color: C.text }}>{text.slice(0, n)}</span>
      {cursor && <span style={{ color: C.blue }}>▋</span>}
    </span>
  );
};

export const Terminal: React.FC<{ children: React.ReactNode; width: number; delay?: number }> = ({ children, width, delay = 0 }) => {
  const p = useIn(delay, 16);
  return (
    <div style={{ width, opacity: p, transform: `scale(${0.9 + 0.1 * p})`, borderRadius: 18, overflow: "hidden", background: C.bg2, boxShadow: "0 30px 80px #0009", border: `2px solid ${C.panel}` }}>
      <div style={{ height: 34, display: "flex", gap: 10, alignItems: "center", paddingLeft: 16, background: C.panel }}>
        {[C.red, C.orange, C.green].map((c) => (
          <div key={c} style={{ width: 14, height: 14, borderRadius: 7, background: c }} />
        ))}
      </div>
      <div style={{ padding: "34px 40px", fontSize: 40, lineHeight: 1.6 }}>{children}</div>
    </div>
  );
};

export const center: React.CSSProperties = { display: "flex", alignItems: "center", justifyContent: "center" };

// ── scenes ──────────────────────────────────────────────────────────────────

type SP = { s: Scene; f: (sec: number) => number };

const TitleScene: React.FC<SP> = () => {
  const p = useIn(5, 10);
  const q = useIn(30);
  return (
    <AbsoluteFill style={{ ...center, flexDirection: "column" }}>
      <Img src={staticFile("logo.webp")} style={{ width: 420, borderRadius: 48, boxShadow: "0 20px 60px #0008", transform: `scale(${0.5 + 0.5 * p})`, opacity: p }} />
      <div style={{ fontFamily: SANS, fontSize: 130, fontWeight: 900, letterSpacing: -3, marginTop: 10, opacity: q, background: `linear-gradient(90deg, ${C.blue}, ${C.purple})`, WebkitBackgroundClip: "text", color: "transparent" }}>
        versaDeploy
      </div>
      <Rise delay={55}>
        <div style={{ fontFamily: SANS, fontSize: 44, color: C.text, marginTop: 10 }}>Deploys atómicos a tu VPS Linux</div>
      </Rise>
      <div style={{ display: "flex", gap: 24, marginTop: 50 }}>
        {["Por SSH", "Sin agentes", "Cero downtime", "Rollback al instante"].map((t, i) => (
          <Chip key={t} text={t} delay={90 + i * 10} size={32} color={[C.blue, C.purple, C.green, C.orange][i]} />
        ))}
      </div>
    </AbsoluteFill>
  );
};

const ProblemScene: React.FC<SP> = ({ s, f }) => {
  const second = f(s.lines[1].start) - 10;
  const frame = useCurrentFrame();
  const fade = interpolate(frame, [second - 10, second + 5], [1, 0.25], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
  return (
    <AbsoluteFill>
      <Title text={s.title!} />
      <div style={{ position: "absolute", top: 300, left: 110, display: "flex", flexDirection: "column", gap: 26, opacity: fade }}>
        {s.items!.map((t, i) => (
          <Rise key={t} delay={15 + i * 12}>
            <div style={{ fontFamily: SANS, fontSize: 42, color: C.text }}>
              <span style={{ color: C.red, marginRight: 22 }}>✗</span>
              {t}
            </div>
          </Rise>
        ))}
      </div>
      <Sequence from={second} layout="none">
        <div style={{ position: "absolute", top: 330, right: 80 }}>
          <Terminal width={800}>
            <Typing text={s.answer!} delay={0} />
            <div style={{ fontFamily: MONO, color: C.green, opacity: interpolate(useCurrentFrame() - second, [45, 60], [0, 1], { extrapolateLeft: "clamp", extrapolateRight: "clamp" }) }}>
              ✓ Deployment successful!
            </div>
          </Terminal>
        </div>
      </Sequence>
    </AbsoluteFill>
  );
};

const PipelineScene: React.FC<SP> = ({ s }) => {
  const frame = useCurrentFrame();
  const { durationInFrames } = useVideoConfig();
  const steps = s.steps!;
  const active = Math.min(steps.length - 1, Math.floor(interpolate(frame, [20, durationInFrames * 0.8], [0, steps.length], { extrapolateLeft: "clamp", extrapolateRight: "clamp" })));
  const colors = [C.cyan, C.orange, C.purple, C.green];
  return (
    <AbsoluteFill>
      <Title text={s.title!} />
      <div style={{ position: "absolute", top: 360, left: 0, right: 0, ...center, gap: 30 }}>
        {steps.map((st, i) => (
          <React.Fragment key={st.name}>
            <Rise delay={10 + i * 10}>
              <div
                style={{
                  width: 360,
                  height: 380,
                  borderRadius: 24,
                  background: C.panel,
                  border: `3px solid ${i <= active ? colors[i] : C.panel}`,
                  boxShadow: i === active ? `0 0 60px ${colors[i]}55` : "none",
                  padding: 34,
                  display: "flex",
                  flexDirection: "column",
                  gap: 18,
                  transform: `scale(${i === active ? 1.04 : 1})`,
                }}
              >
                <div style={{ fontSize: 76 }}>{st.icon}</div>
                <div style={{ fontFamily: SANS, fontSize: 48, fontWeight: 800, color: colors[i] }}>{st.name}</div>
                <div style={{ fontFamily: SANS, fontSize: 31, color: C.text, lineHeight: 1.35 }}>{st.desc}</div>
              </div>
            </Rise>
            {i < steps.length - 1 && <div style={{ fontSize: 56, color: i < active ? C.text : C.muted }}>→</div>}
          </React.Fragment>
        ))}
      </div>
    </AbsoluteFill>
  );
};

const RollbackScene: React.FC<SP> = ({ s, f }) => {
  const frame = useCurrentFrame();
  const fail = 45;
  const back = 90;
  const second = f(s.lines[1].start);
  const releases = ["v1", "v2", "v3"];
  const target = frame < back ? 2 : 1;
  const y = interpolate(frame, [back, back + 20], [2, 1], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
  return (
    <AbsoluteFill>
      <Title text={s.title!} />
      <div style={{ position: "absolute", top: 330, left: 380, display: "flex", flexDirection: "column", gap: 34 }}>
        {releases.map((r, i) => {
          const failed = i === 2 && frame >= fail;
          return (
            <Rise key={r} delay={8 + i * 8}>
              <div style={{ width: 760, whiteSpace: "nowrap", padding: "26px 36px", borderRadius: 18, background: C.panel, fontFamily: MONO, fontSize: 40, color: failed ? C.red : i === target ? C.green : C.text, border: `3px solid ${failed ? C.red : i === target ? C.green : C.panel}` }}>
                releases/{r} {failed ? "✗ health check" : i === target ? "✓" : ""}
              </div>
            </Rise>
          );
        })}
      </div>
      <div style={{ position: "absolute", left: 1190, top: 350 + y * 135, fontFamily: MONO, fontSize: 44, color: C.blue, display: "flex", alignItems: "center", gap: 20 }}>
        ← current
      </div>
      <Sequence from={second} layout="none">
        <div style={{ position: "absolute", bottom: 120, left: 0, right: 0, ...center }}>
          <Terminal width={1000}>
            <Typing text="versa rollback production" delay={0} />
          </Terminal>
        </div>
      </Sequence>
    </AbsoluteFill>
  );
};

const ChipsScene: React.FC<SP & { cols?: number }> = ({ s, cols = 4 }) => {
  const palette = [C.blue, C.purple, C.green, C.orange, C.cyan, C.red];
  return (
    <AbsoluteFill>
      <Title text={s.title!} />
      <div style={{ position: "absolute", top: 320, left: 140, right: 140, display: "grid", gridTemplateColumns: `repeat(${cols}, 1fr)`, gap: 30 }}>
        {s.items!.map((t, i) => (
          <Chip key={t} text={t} delay={10 + i * 6} color={palette[i % palette.length]} size={42} />
        ))}
      </div>
    </AbsoluteFill>
  );
};

const ShotScene: React.FC<SP & { reverse?: boolean }> = ({ s, reverse }) => {
  const palette = [C.green, C.blue, C.purple, C.orange, C.cyan];
  const list = (
    <div style={{ display: "flex", flexDirection: "column", gap: 14, width: 560 }}>
      {s.items!.map((t, i) => (
        <Rise key={t} delay={20 + i * 7}>
          <div style={{ fontFamily: SANS, fontSize: s.items!.length > 6 ? 30 : 36, color: C.text, display: "flex", gap: 16 }}>
            <span style={{ color: palette[i % palette.length] }}>●</span>
            {t}
          </div>
        </Rise>
      ))}
    </div>
  );
  return (
    <AbsoluteFill>
      <Title text={s.title!} />
      <div style={{ position: "absolute", top: 270, left: 90, right: 90, display: "flex", gap: 60, alignItems: "flex-start", flexDirection: reverse ? "row-reverse" : "row" }}>
        <Window src={`shots/${s.shot}.png`} width={1150} delay={8} />
        {list}
      </div>
    </AbsoluteFill>
  );
};

const SecurityScene: React.FC<SP> = ({ s }) => (
  <AbsoluteFill>
    <Title text={s.title!} />
    <div style={{ position: "absolute", top: 330, left: 220, right: 220, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 40 }}>
      {s.items!.map((t, i) => (
        <Rise key={t} delay={10 + i * 10}>
          <div style={{ background: C.panel, borderRadius: 22, padding: "40px 44px", display: "flex", gap: 28, alignItems: "center", fontFamily: SANS, fontSize: 40, color: C.text }}>
            <span style={{ fontSize: 60 }}>🔒</span>
            {t}
          </div>
        </Rise>
      ))}
    </div>
  </AbsoluteFill>
);

const InstallScene: React.FC<SP> = ({ s, f }) => (
  <AbsoluteFill>
    <Title text={s.title!} />
    <div style={{ position: "absolute", top: 360, left: 0, right: 0, ...center }}>
      <Terminal width={1680} delay={5}>
        <div style={{ fontSize: 27 }}>
          <Typing text={s.code!} delay={20} cps={45} />
        </div>
      </Terminal>
    </div>
    <div style={{ position: "absolute", top: 700, left: 0, right: 0, ...center, gap: 30 }}>
      {["Linux", "macOS", "Windows"].map((t, i) => (
        <Chip key={t} text={t} delay={60 + i * 8} color={[C.orange, C.blue, C.cyan][i]} />
      ))}
    </div>
    <Sequence from={f(s.lines[1].start)} layout="none">
      <div style={{ position: "absolute", top: 860, left: 0, right: 0, ...center, gap: 30 }}>
        {["Gratis", "Código abierto", "Licencia MIT"].map((t, i) => (
          <Chip key={t} text={t} delay={i * 8} color={C.green} size={36} />
        ))}
      </div>
    </Sequence>
  </AbsoluteFill>
);

const OutroScene: React.FC<SP> = ({ s }) => {
  const p = useIn(0, 12);
  return (
    <AbsoluteFill style={{ ...center, flexDirection: "column" }}>
      <Img src={staticFile("logo.webp")} style={{ width: 300, borderRadius: 36, boxShadow: "0 20px 60px #0008", opacity: p, transform: `scale(${0.6 + 0.4 * p})` }} />
      <div style={{ fontFamily: SANS, fontSize: 110, fontWeight: 900, background: `linear-gradient(90deg, ${C.blue}, ${C.purple})`, WebkitBackgroundClip: "text", color: "transparent", opacity: p }}>
        {s.title}
      </div>
      <Rise delay={20}>
        <div style={{ fontFamily: MONO, fontSize: 44, color: C.cyan }}>{s.url}</div>
      </Rise>
      <Rise delay={40} style={{ marginTop: 60 }}>
        <div style={{ fontFamily: SANS, fontSize: 36, color: C.muted, textAlign: "center", marginBottom: 24 }}>Próximos videos</div>
      </Rise>
      <div style={{ display: "flex", gap: 28 }}>
        {s.next!.map((t, i) => (
          <Chip key={t} text={t} delay={50 + i * 10} color={[C.red, C.green, C.green][i]} />
        ))}
      </div>
    </AbsoluteFill>
  );
};

const SCENES: Record<string, React.FC<SP>> = {
  title: TitleScene,
  problem: ProblemScene,
  pipeline: PipelineScene,
  rollback: RollbackScene,
  frameworks: (p) => <ChipsScene {...p} cols={4} />,
  services: (p) => <ShotScene {...p} />,
  distros: (p) => <ChipsScene {...p} cols={3} />,
  tui: (p) => <ShotScene {...p} reverse />,
  team: (p) => <ShotScene {...p} />,
  security: SecurityScene,
  install: InstallScene,
  outro: OutroScene,
};

// ── captions, background, audio ─────────────────────────────────────────────

const Captions: React.FC<{ t: Timeline }> = ({ t }) => {
  const frame = useCurrentFrame();
  const sec = frame / t.fps;
  for (const s of t.scenes) {
    for (const l of s.lines) {
      const a = s.start + l.start;
      if (sec >= a && sec < a + l.dur + 0.3) {
        return (
          <div style={{ position: "absolute", bottom: 40, left: 0, right: 0, ...center }}>
            <div style={{ maxWidth: 1500, textAlign: "center", fontFamily: SANS, fontSize: 40, color: "#fff", background: "#000b", padding: "12px 26px", borderRadius: 10, lineHeight: 1.35 }}>
              {l.text}
            </div>
          </div>
        );
      }
    }
  }
  return null;
};

export const Background: React.FC = () => {
  const frame = useCurrentFrame();
  const x = Math.sin(frame / 200) * 200;
  return (
    <AbsoluteFill style={{ background: C.bg }}>
      <AbsoluteFill style={{ background: `radial-gradient(1000px 700px at ${1300 + x}px 200px, ${C.purple}22, transparent), radial-gradient(900px 600px at ${400 - x}px 900px, ${C.blue}1f, transparent)` }} />
      <AbsoluteFill style={{ backgroundImage: `linear-gradient(${C.panel}55 1px, transparent 1px), linear-gradient(90deg, ${C.panel}55 1px, transparent 1px)`, backgroundSize: "80px 80px", opacity: 0.35 }} />
    </AbsoluteFill>
  );
};

export const Intro: React.FC<{ timeline: Timeline; captions: boolean }> = ({ timeline: t, captions }) => {
  const { durationInFrames } = useVideoConfig();
  const fps = t.fps;
  // the music drops under the voice (ducking) and fades in and out
  const speaking = (frame: number) => {
    const sec = frame / fps;
    let d = 99;
    for (const s of t.scenes)
      for (const l of s.lines) {
        const a = s.start + l.start;
        const b = a + l.dur;
        d = Math.min(d, sec < a ? a - sec : sec > b ? sec - b : 0);
      }
    return d;
  };
  const musicVolume = (frame: number) => {
    const duck = interpolate(speaking(frame), [0, 0.5], [0.07, 0.24], { extrapolateRight: "clamp" });
    const fade = Math.min(interpolate(frame, [0, 2 * fps], [0, 1], { extrapolateRight: "clamp" }), interpolate(frame, [durationInFrames - 3 * fps, durationInFrames], [1, 0], { extrapolateLeft: "clamp" }));
    return duck * fade;
  };
  return (
    <AbsoluteFill>
      <Background />
      <Audio src={staticFile("music.m4a")} loop volume={musicVolume} />
      {t.scenes.map((s) => {
        const Comp = SCENES[s.id];
        const from = Math.round(s.start * fps);
        const local = (sec: number) => Math.round(sec * fps);
        return (
          <Sequence key={s.id} from={from} durationInFrames={Math.round(s.dur * fps)}>
            <Fade dur={Math.round(s.dur * fps)}>
              <Comp s={s} f={local} />
            </Fade>
            {s.lines.map((l) => (
              <Sequence key={l.file} from={local(l.start)} layout="none">
                <Audio src={staticFile(l.file)} volume={1} />
              </Sequence>
            ))}
          </Sequence>
        );
      })}
      {captions && <Captions t={t} />}
    </AbsoluteFill>
  );
};

export const Fade: React.FC<{ dur: number; children: React.ReactNode }> = ({ dur, children }) => {
  const frame = useCurrentFrame();
  const o = Math.min(interpolate(frame, [0, 12], [0, 1], { extrapolateRight: "clamp" }), interpolate(frame, [dur - 12, dur], [1, 0], { extrapolateLeft: "clamp" }));
  return <AbsoluteFill style={{ opacity: o }}>{children}</AbsoluteFill>;
};
