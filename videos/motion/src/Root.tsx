import React from "react";
import { Composition, staticFile } from "remotion";
import { Intro, Timeline } from "./Intro";
import { Episode, EpisodeTimeline } from "./Episode";
import timeline from "../public/timeline.json";

const t = timeline as Timeline;

export const Root: React.FC = () => (
  <>
    <Composition
      id="Intro"
      component={Intro}
      durationInFrames={Math.ceil(t.total * t.fps)}
      fps={t.fps}
      width={1920}
      height={1080}
      defaultProps={{ timeline: t, captions: false }}
    />
    {/* an episode: --props='{"ep":"02","captions":true}' (timeline from episode.py) */}
    <Composition
      id="Episode"
      component={Episode}
      durationInFrames={30}
      fps={30}
      width={1920}
      height={1080}
      defaultProps={{ ep: "02", captions: false, timeline: null as EpisodeTimeline | null }}
      calculateMetadata={async ({ props }) => {
        const tl = (await (await fetch(staticFile(`ep/${props.ep}.json`))).json()) as EpisodeTimeline;
        return { durationInFrames: Math.ceil(tl.total * tl.fps), fps: tl.fps, props: { ...props, timeline: tl } };
      }}
    />
  </>
);
