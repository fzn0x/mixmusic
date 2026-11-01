// The API plugins get, built over the natives of the Go side (native), as
// the web interface's src/plugins/api.ts built it over its own. It returns
// the MixMusicAPI of sdk/index.d.ts, the types published for plugin
// authors, which the type checker holds it to; docs/plugins.md tells how
// to use it. `bun run build:plugins` compiles it to api.js, which the Go
// side runs: a script, not a module, whose value is this function.

type API = import("../../sdk").MixMusicAPI;
type Track = import("../../sdk").Track;

/** What the Go side gives the API to build on (natives.go). */
interface Natives {
  readonly version: string;
  readonly plugin: string;
  playerState(): ReturnType<API["player"]["state"]>;
  play(): void;
  pause(): void;
  toggle(): void;
  next(): void;
  previous(): void;
  seek(ms: number): void;
  setVolume(volume: number): void;
  playTracks(tracks: Track[], start: number): void;
  playNext(tracks: Track[]): void;
  enqueue(tracks: Track[]): void;
  on(event: string, listener: () => void): () => void;
  analyser(): ReturnType<API["player"]["analyser"]>;
  search(query: string): ReturnType<API["catalog"]["search"]>;
  album(id: string): ReturnType<API["catalog"]["album"]>;
  artist(id: string): ReturnType<API["catalog"]["artist"]>;
  playlist(id: string): ReturnType<API["catalog"]["playlist"]>;
  radio(track: Track): ReturnType<API["catalog"]["radio"]>;
  lyrics(track: Track): ReturnType<API["catalog"]["lyrics"]>;
  isLiked(id: string): boolean;
  setLiked(track: Track, liked: boolean): Promise<void>;
  mixes(): ReturnType<API["library"]["mixes"]>;
  createMix(title: string, tracks: Track[]): ReturnType<API["library"]["createMix"]>;
  toast(message: string, tone: string): void;
  navigate(path: string): void;
  theme(): ReturnType<API["ui"]["theme"]>;
  addSidebarItem(id: string, label: string, icon: string, page: string): () => void;
  addPage(name: string, title: string, render: Parameters<API["ui"]["addPage"]>[0]["render"]): () => void;
  addTrackAction(id: string, label: string, run: (track: Track) => void): () => void;
  addWidget(id: string, title: string, render: Parameters<API["ui"]["addWidget"]>[0]["render"]): () => void;
  registerCommand(id: string, title: string, shortcut: string, run: () => void): () => void;
  settings(): ReturnType<API["settings"]["get"]>;
  storageGet(key: string): Promise<string | undefined>;
  storageSet(key: string, value: string): Promise<void>;
  storageDelete(key: string): Promise<void>;
  notify(title: string, body: string): Promise<void>;
  openExternal(url: string): Promise<void>;
}

(function (native: Natives): API {
  const done = Promise.resolve();
  const call =
    <A extends unknown[]>(fn: (...args: A) => unknown) =>
    (...args: A) => {
      fn(...args);
      return done;
    };
  // The elements a page or a widget shows: plain objects, which plugins may
  // write themselves.
  const elements = {
    text: (text, options) => ({ ...options, type: "text", text: String(text) }),
    heading: (text) => ({ type: "heading", text: String(text) }),
    button: (label, onClick, options) => ({ ...options, type: "button", label: String(label), onClick }),
    link: (label, target) =>
      typeof target === "function" ? { type: "link", label: String(label), onClick: target } : { type: "link", label: String(label), path: String(target) },
    tiles: (children) => ({ type: "tiles", children }),
    tile: (title, onClick, options) => ({ ...options, type: "tile", title: String(title), onClick }),
    row: (children, options) => ({ ...options, type: "row", children }),
    column: (children, options) => ({ ...options, type: "column", children }),
    canvas: (options) => ({ ...options, type: "canvas" }),
    image: (url, options) => ({ ...options, type: "image", url: String(url) }),
    spacer: (size) => ({ type: "spacer", size }),
    toggle: (label, on, onChange) => ({ type: "toggle", label: String(label), on: Boolean(on), onChange }),
    input: (options) => ({ ...options, type: "input" }),
  } satisfies Partial<API["ui"]>;
  const getJSON = async <T,>(key: string, fallback: T): Promise<T> => {
    const raw = await native.storageGet(String(key));
    if (raw == null) return fallback;
    try {
      return JSON.parse(raw);
    } catch {
      return fallback;
    }
  };
  return Object.freeze({
    version: native.version,
    plugin: native.plugin,
    player: Object.freeze({
      state: () => native.playerState(),
      play: call(native.play),
      pause: call(native.pause),
      toggle: call(native.toggle),
      next: call(native.next),
      previous: call(native.previous),
      seek: call((ms: number) => native.seek(Number(ms))),
      setVolume: (volume: number) => native.setVolume(Number(volume)),
      playTracks: call((tracks: Track[], start?: number) => native.playTracks(tracks, Number(start ?? 0))),
      playNext: (tracks: Track[]) => native.playNext(tracks),
      enqueue: (tracks: Track[]) => native.enqueue(tracks),
      on: (event: string, listener: () => void) => native.on(String(event), listener),
      analyser: () => native.analyser(),
    }),
    catalog: Object.freeze({
      search: (query: string) => native.search(String(query)),
      album: (id: string) => native.album(String(id)),
      artist: (id: string) => native.artist(String(id)),
      playlist: (id: string) => native.playlist(String(id)),
      radio: (track: Track) => native.radio(track),
      lyrics: (track: Track) => native.lyrics(track),
    }),
    library: Object.freeze({
      isLiked: (id: string) => native.isLiked(String(id)),
      setLiked: (track: Track, liked: boolean) => native.setLiked(track, Boolean(liked)),
      mixes: () => native.mixes(),
      createMix: (title: string, tracks?: Track[]) => native.createMix(String(title), tracks ?? []),
    }),
    ui: Object.freeze({
      toast: (message: string, options?: { tone?: string }) => native.toast(String(message), String(options?.tone ?? "info")),
      navigate: (path: string) => native.navigate(String(path)),
      theme: () => native.theme(),
      addSidebarItem: ({ id, label, icon, page }: Parameters<API["ui"]["addSidebarItem"]>[0]) =>
        native.addSidebarItem(String(id), String(label), String(icon ?? ""), String(page)),
      addPage: ({ name, title, render }: Parameters<API["ui"]["addPage"]>[0]) => native.addPage(String(name), String(title), render),
      addTrackAction: ({ id, label, run }: Parameters<API["ui"]["addTrackAction"]>[0]) => native.addTrackAction(String(id), String(label), run),
      addWidget: ({ id, title, render }: Parameters<API["ui"]["addWidget"]>[0]) => native.addWidget(String(id), String(title), render),
      ...elements,
    }),
    commands: Object.freeze({
      register: ({ id, title, shortcut, run }: Parameters<API["commands"]["register"]>[0]) =>
        native.registerCommand(String(id), String(title), String(shortcut ?? ""), run),
    }),
    settings: Object.freeze({
      get: () => native.settings(),
    }),
    storage: Object.freeze({
      get: (key: string) => native.storageGet(String(key)),
      set: (key: string, value: string) => native.storageSet(String(key), String(value)),
      delete: (key: string) => native.storageDelete(String(key)),
      getJSON,
      setJSON: (key: string, value: unknown) => native.storageSet(String(key), JSON.stringify(value)),
    }),
    notify: (title: string, body: string) => native.notify(String(title), String(body)),
    openExternal: (url: string) => native.openExternal(String(url)),
  });
});
