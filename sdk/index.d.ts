// Types of the MixMusic plugin API, version 2, for plugin authors: that of
// MixMusic's own interface, which a plugin declares with "api": 2 in its
// plugin.json.
//
// A plugin's module exports a default function that MixMusic calls with the
// API when the user enables the plugin. It may return a function, which
// MixMusic calls when the plugin is disabled, updated or removed:
//
//   import type { MixMusicAPI } from "./sdk";
//
//   export default function activate(api: MixMusicAPI) {
//     const off = api.player.on("trackchange", () => console.log(api.player.state().track?.title));
//     return off;
//   }
//
// MixMusic runs the JavaScript a plugin's TypeScript compiles to.
// Everything a plugin adds through the API (sidebar items, pages, track
// actions, widgets, commands and listeners) is removed for it when it
// stops, and its timers stop. See docs/plugins.md.

/** A service tracks come from; "local" is a mix made in MixMusic. */
export type Source = "youtube" | "spotify" | "local";

export interface Image {
  url: string;
  width?: number;
  height?: number;
}

/** A link to an artist or an album; id is set when the service has a page for it. */
export interface Ref {
  id?: string;
  name: string;
}

/** A track. Its id starts with its service: "yt:" for YouTube Music, "sp:" for Spotify. */
export interface Track {
  id: string;
  source: Source;
  title: string;
  artists: Ref[];
  album?: Ref | null;
  durationMs: number;
  artwork: Image[];
  explicit?: boolean;
  video?: boolean;
  url?: string;
}

export interface Album {
  id: string;
  source: Source;
  title: string;
  artists: Ref[];
  kind?: "album" | "single" | "ep";
  year?: string;
  artwork: Image[];
  trackCount?: number;
  tracks: Track[];
  url?: string;
}

export interface Artist {
  id: string;
  source: Source;
  name: string;
  artwork: Image[];
  subtitle?: string;
  description?: string;
  topTracks: Track[];
  albums: Album[];
  singles: Album[];
  related: Ref[];
  url?: string;
}

export interface Playlist {
  id: string;
  source: Source;
  title: string;
  description?: string;
  owner?: string;
  artwork: Image[];
  trackCount?: number;
  tracks: Track[];
  editable?: boolean;
  url?: string;
}

export interface SearchResults {
  tracks: Track[];
  albums: Album[];
  artists: Artist[];
  playlists: Playlist[];
}

export interface SearchResponse {
  youtube: SearchResults;
  spotify: SearchResults;
  spotifySignedIn: boolean;
  errors: string[];
}

export interface Lyrics {
  synced: boolean;
  instrumental?: boolean;
  /** Lines of the lyrics; timeMs is when a line is sung, when synced. */
  lines: { timeMs: number; text: string }[];
  source: string;
}

export interface PlayerSnapshot {
  track: Track | null;
  playing: boolean;
  positionMs: number;
  durationMs: number;
  /** From 0 to 1. */
  volume: number;
  shuffle: boolean;
  repeat: "off" | "all" | "one";
  queue: Track[];
  /** The position of the current track in the queue, or -1. */
  index: number;
}

/** What plugins may hear of the player. */
export type PlayerEvent = "trackchange" | "statechange" | "progress" | "queuechange";

/** Plugins' render functions may return a cleanup function. */
export type Cleanup = void | (() => void);

/** What pages and widgets show: elements, which api.ui's functions make, or plain objects of their shape. null, undefined and false show nothing, and arrays are flattened. */
export type Element =
  | { type: "text"; text: string; tone?: "muted" | "accent" | "danger"; size?: "small" | "body" | "subtitle" | "title" }
  | { type: "heading"; text: string }
  | { type: "button"; label: string; onClick?: () => void; primary?: boolean }
  | { type: "link"; label: string; onClick?: () => void; path?: string }
  | { type: "tiles"; children: Children }
  | { type: "tile"; title: string; onClick?: () => void; tone?: number; caption?: string }
  | { type: "row" | "column"; children: Children; gap?: number }
  | { type: "toggle"; label: string; on: boolean; onChange?: (on: boolean) => void }
  | { type: "input"; placeholder?: string; value?: string; onSubmit?: (text: string) => void }
  | { type: "image"; url: string; size?: number; round?: boolean }
  | { type: "spacer"; size: number }
  | { type: "canvas"; width?: number; height?: number; animate?: boolean; draw(ctx: Context2D, width: number, height: number): void };

export type Children = Element | string | null | undefined | false | Children[];

/** The view a page or a widget renders into: set replaces what it shows. */
export interface View {
  set(...elements: Children[]): void;
  clear(): void;
}

/** The part of the web's CanvasRenderingContext2D a canvas draws with; sizes are in the interface's units, as CSS pixels. */
export interface Context2D {
  readonly canvas: { readonly width: number; readonly height: number };
  fillStyle: string;
  strokeStyle: string;
  lineWidth: number;
  lineCap: string;
  globalAlpha: number;
  save(): void;
  restore(): void;
  translate(x: number, y: number): void;
  rotate(angle: number): void;
  scale(x: number, y: number): void;
  transform(a: number, b: number, c: number, d: number, e: number, f: number): void;
  setTransform(a: number, b: number, c: number, d: number, e: number, f: number): void;
  resetTransform(): void;
  beginPath(): void;
  moveTo(x: number, y: number): void;
  lineTo(x: number, y: number): void;
  arc(x: number, y: number, radius: number, startAngle: number, endAngle: number, counterclockwise?: boolean): void;
  rect(x: number, y: number, width: number, height: number): void;
  quadraticCurveTo(cpx: number, cpy: number, x: number, y: number): void;
  bezierCurveTo(cp1x: number, cp1y: number, cp2x: number, cp2y: number, x: number, y: number): void;
  closePath(): void;
  stroke(): void;
  fill(): void;
  fillRect(x: number, y: number, width: number, height: number): void;
  strokeRect(x: number, y: number, width: number, height: number): void;
  /** Clearing the whole canvas drops what was drawn before; every frame starts clear. */
  clearRect(x: number, y: number, width: number, height: number): void;
}

/** The reading methods and properties of the web's AnalyserNode, of what MixMusic's own player plays. */
export interface Analyser {
  readonly fftSize: number;
  readonly frequencyBinCount: number;
  readonly minDecibels: number;
  readonly maxDecibels: number;
  readonly smoothingTimeConstant: number;
  getByteFrequencyData(array: Uint8Array): void;
  getFloatFrequencyData(array: Float32Array): void;
  getByteTimeDomainData(array: Uint8Array): void;
  getFloatTimeDomainData(array: Float32Array): void;
}

/** The colours of the theme, as CSS hex. */
export interface Theme {
  accent: string;
  accentText: string;
  text: string;
  muted: string;
  background: string;
  surface: string;
  tiles: [string, string, string, string];
  tileText: string;
  dark: boolean;
}

export interface MixMusicAPI {
  /** MixMusic's version. */
  readonly version: string;
  /** The plugin's own id. */
  readonly plugin: string;

  player: {
    state(): PlayerSnapshot;
    play(): Promise<void>;
    pause(): Promise<void>;
    toggle(): Promise<void>;
    next(): Promise<void>;
    previous(): Promise<void>;
    seek(positionMs: number): Promise<void>;
    /** From 0 to 1. */
    setVolume(volume: number): void;
    /** Replaces the queue with tracks and plays the one at start. */
    playTracks(tracks: Track[], start?: number): Promise<void>;
    /** Puts tracks right after the current one. */
    playNext(tracks: Track[]): void;
    /** Adds tracks at the end of the queue. */
    enqueue(tracks: Track[]): void;
    /** Listens to the player; returns the function that stops listening. */
    on(event: PlayerEvent, listener: () => void): () => void;
    /** The analyser of MixMusic's own player, of fftSize 256: silence while it plays nothing. Spotify's audio, played on a Spotify Connect device, does not reach it. */
    analyser(): Analyser;
  };

  catalog: {
    /** Searches YouTube Music, and Spotify when the user signed in. */
    search(query: string): Promise<SearchResponse>;
    album(id: string): Promise<Album>;
    artist(id: string): Promise<Artist>;
    playlist(id: string): Promise<Playlist>;
    /** Tracks like the one given. */
    radio(track: Track): Promise<Track[]>;
    lyrics(track: Track): Promise<Lyrics>;
  };

  library: {
    isLiked(trackId: string): boolean;
    setLiked(track: Track, liked: boolean): Promise<void>;
    /** The playlists made in MixMusic. */
    mixes(): Playlist[];
    createMix(title: string, tracks?: Track[]): Promise<Playlist>;
  };

  ui: {
    toast(message: string, options?: { tone?: "info" | "error" | "success" }): void;
    /** Shows a page of the app, as "/search?q=jazz" or "/plugin/<id>/<page>". */
    navigate(path: string): void;
    /** Adds an item to the sidebar, opening one of the plugin's pages. icon is an SVG string of a 24×24 icon drawn in currentColor. */
    addSidebarItem(item: { id: string; label: string; icon?: string; page: string }): () => void;
    /** Adds a page at /plugin/<plugin id>/<name>, rendered into a container. */
    addPage(page: { name: string; title: string; render(view: View): Cleanup }): () => void;
    /** Adds an item to the menu of every track. */
    addTrackAction(action: { id: string; label: string; run(track: Track): void }): () => void;
    /** Adds a section to the now playing page. */
    addWidget(widget: { id: string; title: string; render(view: View): Cleanup }): () => void;
    /** The colours of the theme, which pages and canvases draw in. */
    theme(): Theme;
    text(text: string, options?: { tone?: "muted" | "accent" | "danger"; size?: "small" | "body" | "subtitle" | "title" }): Element;
    heading(text: string): Element;
    button(label: string, onClick: () => void, options?: { primary?: boolean }): Element;
    /** A link that does something, or shows a page of the app, as "/search?q=jazz". */
    link(label: string, target: (() => void) | string): Element;
    tiles(tiles: Children): Element;
    tile(title: string, onClick: () => void, options?: { tone?: number; caption?: string }): Element;
    row(children: Children, options?: { gap?: number }): Element;
    column(children: Children, options?: { gap?: number }): Element;
    /** A switch, which shows on: onChange gets what the user chose, which shows once the view is set again. */
    toggle(label: string, on: boolean, onChange: (on: boolean) => void): Element;
    input(options: { placeholder?: string; value?: string; onSubmit(text: string): void }): Element;
    image(url: string, options?: { size?: number; round?: boolean }): Element;
    spacer(size: number): Element;
    /** A drawing, which draw makes, at every frame while animate is set, unless the computer asks for less motion. */
    canvas(options: { width?: number; height?: number; animate?: boolean; draw(ctx: Context2D, width: number, height: number): void }): Element;
  };

  commands: {
    /** Adds a command to the command palette, with an optional shortcut as "Mod+Shift+P" (Mod is Ctrl, or Cmd on macOS). */
    register(command: { id: string; title: string; shortcut?: string; run(): void }): () => void;
  };

  settings: {
    /** The user's settings, read only. */
    get(): Readonly<Record<string, unknown>>;
  };

  /** Values the plugin keeps between runs, up to 1 MB. */
  storage: {
    get(key: string): Promise<string | undefined>;
    set(key: string, value: string): Promise<void>;
    delete(key: string): Promise<void>;
    getJSON<T>(key: string, fallback: T): Promise<T>;
    setJSON(key: string, value: unknown): Promise<void>;
  };

  /** Shows a desktop notification. */
  notify(title: string, body: string): Promise<void>;
  /** Opens a web page in the default browser. */
  openExternal(url: string): Promise<void>;
}

/** The function a plugin's module exports by default. */
export type Activate = (api: MixMusicAPI) => void | (() => void) | Promise<void | (() => void)>;
