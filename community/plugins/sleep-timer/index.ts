// Sleep Timer: fades the music out and pauses it after a while, or at the
// end of the track. It adds a page to the sidebar and commands to the
// command palette.

import type { MixMusicAPI } from "../../../sdk";

/** How long the music fades out before it pauses, in milliseconds. */
const FADE_MS = 30_000;

const icon =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="square"><path d="M19.5 14.5A8 8 0 1 1 9.5 4.5a6.5 6.5 0 0 0 10 10Z"/></svg>';

export default function activate(api: MixMusicAPI) {
  let deadline = 0; // when the music pauses, or 0
  let atTrackEnd = false;
  let tick: ReturnType<typeof setInterval> | null = null;
  let fadeFrom: number | null = null; // the volume before the fade
  let offTrack: (() => void) | null = null;
  const listeners = new Set<() => void>();
  const changed = () => listeners.forEach((fn) => fn());

  function clear() {
    if (tick != null) clearInterval(tick);
    tick = null;
    offTrack?.();
    offTrack = null;
    if (fadeFrom != null) api.player.setVolume(fadeFrom);
    fadeFrom = null;
    deadline = 0;
    atTrackEnd = false;
    changed();
  }

  function stopMusic() {
    const volume = fadeFrom;
    clear();
    api.player.pause().then(() => volume != null && api.player.setVolume(volume));
    api.ui.toast("Sleep timer: good night");
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
    api.storage.setJSON("last", minutes);
    api.ui.toast(`Sleep timer: the music stops in ${minutes} minutes`);
    changed();
  }

  function startAtTrackEnd() {
    clear();
    atTrackEnd = true;
    const current = api.player.state().track?.id;
    offTrack = api.player.on("trackchange", () => {
      if (api.player.state().track?.id !== current) stopMusic();
    });
    api.ui.toast("Sleep timer: the music stops after this track");
    changed();
  }

  function describe() {
    if (atTrackEnd) return "stops after this track";
    if (!deadline) return "off";
    const s = Math.max(0, Math.round((deadline - Date.now()) / 1000));
    return `stops in ${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
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
          text(`The sleep timer is ${describe()}.`, { size: "subtitle" }),
          tiles([15, 30, 60, 90].map((m, i) => tile(`${m} min`, () => startMinutes(m), { tone: (i % 4) + 1 }))),
          row([button("at the end of this track", startAtTrackEnd), button("cancel", clear)]),
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
