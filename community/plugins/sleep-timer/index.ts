// Sleep Timer: fades the music out and pauses it after a while, or at the
// end of the track. It adds a page to the sidebar and commands to the
// command palette.

import type { MixMusicAPI } from "../../../sdk";

/** How long the music fades out before it pauses, in milliseconds. */
const FADE_MS = 30_000;

/** How close to the end of a track the music pauses, in milliseconds of what is heard. */
const END_MS = 1000;

/** How many times, 500 ms apart, the pause is sent again: a track that loads ignores it. */
const PAUSE_TRIES = 8;

const icon =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="square"><path d="M19.5 14.5A8 8 0 1 1 9.5 4.5a6.5 6.5 0 0 0 10 10Z"/></svg>';

export default function activate(api: MixMusicAPI) {
  let deadline = 0; // when the music pauses, or 0
  let atTrackEnd = false;
  let tick: ReturnType<typeof setInterval> | null = null;
  let fadeFrom: number | null = null; // the volume before the fade
  let offTrack: (() => void) | null = null;
  let offProgress: (() => void) | null = null;
  let left = Infinity; // what the track had left, heard, at the last progress
  const listeners = new Set<() => void>();
  const changed = () => listeners.forEach((fn) => fn());

  function clear(restore = true) {
    if (tick != null) clearInterval(tick);
    tick = null;
    offTrack?.();
    offTrack = null;
    offProgress?.();
    offProgress = null;
    if (restore && fadeFrom != null) api.player.setVolume(fadeFrom);
    fadeFrom = null;
    deadline = 0;
    atTrackEnd = false;
    changed();
  }

  function stopMusic() {
    const volume = fadeFrom;
    clear(false);
    api.ui.toast("Sleep timer: good night");
    // A pause does nothing while a track loads, so it is sent again until
    // the music has stopped. The volume comes back after it, so that a
    // device doesn't turn up first.
    let tries = 0;
    const settle = () => {
      api.player.pause().then(() => {
        setTimeout(() => {
          tries++;
          if ((tries >= 2 && !api.player.state().playing) || tries >= PAUSE_TRIES) {
            if (volume != null) api.player.setVolume(volume);
          } else settle();
        }, 500);
      });
    };
    settle();
  }

  function step() {
    const left = deadline - Date.now();
    if (left <= 0) return stopMusic();
    if (left < FADE_MS) {
      if (fadeFrom == null) fadeFrom = api.player.state().volume;
      api.player.setVolume(fadeFrom * (left / FADE_MS));
    }
    changed();
  }

  function startMinutes(minutes: number) {
    clear();
    deadline = Date.now() + minutes * 60_000;
    tick = setInterval(step, 1000);
    api.ui.toast(`Sleep timer: the music stops in ${minutes} minutes`);
    changed();
  }

  /** The seconds a crossfade overlaps the tracks by, which the pause comes before. */
  function crossfadeMs() {
    const seconds = Number(api.settings.get().crossfade) || 0;
    return api.player.state().queue.length > 1 ? seconds * 1000 : 0;
  }

  function startAtTrackEnd() {
    clear();
    atTrackEnd = true;
    left = Infinity;
    // The music pauses as the track nears its end, before a crossfade or the
    // next track starts: a pause that comes with the next track is dropped
    // while it loads. A repeated track, or one queued twice, has the same ID,
    // and a queue's last track sends no trackchange.
    offProgress = api.player.on("progress", () => {
      const s = api.player.state();
      if (!s.playing || s.durationMs <= 0) return;
      left = (s.durationMs - s.positionMs) / (s.speed || 1);
      if (left <= END_MS + crossfadeMs()) stopMusic();
    });
    // A new track: the end came between two progress events, which is so
    // when the last one was close to it, and a press of Next is not the end.
    offTrack = api.player.on("trackchange", () => {
      if (left <= END_MS + crossfadeMs() + 2000) stopMusic();
      else left = Infinity;
    });
    api.ui.toast("Sleep timer: the music stops after this track");
    changed();
  }

  function describe() {
    if (atTrackEnd) return "The sleep timer stops after this track.";
    if (!deadline) return "The sleep timer is off.";
    const s = Math.max(0, Math.round((deadline - Date.now()) / 1000));
    return `The sleep timer stops in ${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}.`;
  }

  for (const minutes of [15, 30, 45, 60, 90]) {
    api.commands.register({ id: `m${minutes}`, title: `Sleep timer: ${minutes} minutes`, run: () => startMinutes(minutes) });
  }
  api.commands.register({ id: "track", title: "Sleep timer: at the end of this track", run: startAtTrackEnd });
  api.commands.register({ id: "cancel", title: "Sleep timer: cancel", run: () => (clear(), api.ui.toast("Sleep timer cancelled")) });

  api.ui.addSidebarItem({ id: "page", label: "sleep timer", icon, page: "timer" });
  api.ui.addPage({
    name: "timer",
    title: "sleep timer",
    render(view) {
      const { text, tiles, tile, row, button } = api.ui;
      const render = () =>
        view.set(
          text(describe(), { size: "subtitle" }),
          tiles([15, 30, 45, 60, 90].map((m, i) => tile(`${m} min`, () => startMinutes(m), { tone: (i % 4) + 1 }))),
          row([button("at the end of this track", startAtTrackEnd), button("cancel", () => clear())]),
        );
      listeners.add(render);
      const refresh = setInterval(render, 1000);
      render();
      return () => {
        listeners.delete(render);
        clearInterval(refresh);
      };
    },
  });

  return clear;
}
