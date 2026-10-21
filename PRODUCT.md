# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

The interface is drawn by the desktop app itself, on the GPU, with MyGo's native UI: no web page. It is a desktop app first: a resizable window from about 940×600 upward, mouse and keyboard, a tray icon, media keys, and a frameless mini player window.

## Users

Power listeners: people who use both Spotify and YouTube Music every day, who want one player for both, and who care about tuning that player to their taste. They search fast, queue a lot, live with the app open for hours, use the keyboard, and install, tweak or write themes and plugins. Many of them are on Windows; a meaningful share are on Linux desktops.

## Product Purpose

MixMusic plays Spotify and YouTube Music in one place: one search across both services, one queue that mixes tracks from both, one library of liked songs and playlists that can hold tracks of either service. Success is a listener who stops switching between two apps and keeps MixMusic open as their only player.

## Positioning

One queue and one library across Spotify and YouTube Music, in an open-source desktop player whose whole look and behavior the community can change through themes and plugins installed from a shared registry.

## Operating Context

- Spotify tracks play in MixMusic itself through Spotify's Web Playback SDK, by default on Windows, where the webview plays Spotify's protected audio, or on a Spotify Connect device (the user's own Spotify app, a phone, a speaker), the default elsewhere; both need Premium. Matching their recording on YouTube Music and playing it in MixMusic's own player is the alternative, and the fallback when Spotify cannot play.
- YouTube Music needs no account. Its audio streams are found by yt-dlp, which MixMusic installs on request, with a JavaScript runtime (Deno, Node, Bun or QuickJS) found on the system. Signing in to YouTube Music, optional, happens on Google's own page in a window of MixMusic; MixMusic keeps the session's cookies encrypted, and YouTube does not offer this to apps officially.
- Spotify sign-in uses each user's own Client ID from the Spotify developer dashboard, with the redirect URI `http://127.0.0.1:47823/callback`. Spotify's development mode limits apps: an owner with Premium, up to five users added under Users Management (refused otherwise with a 403 that Settings › Accounts explains), ten search results per kind, no access to the tracks of playlists the user does not own, no artist top-tracks endpoint.
- Lyrics come from LRCLIB (synchronized when available) and YouTube Music.
- The app runs for hours in the background: the tray, media keys and the system's media controls, desktop notifications, the mini player. It can scrobble to Last.fm (with the user's own API account), show what plays on Discord (with the user's Discord application), and update itself from signed releases.

## Capabilities and Constraints

- Search across both services; home shelves from both plus the user's own; album, artist and playlist pages; radio continuation when the queue ends.
- Unified queue, shuffle, repeat, crossfade, volume levelling, a 10-band equalizer, synced lyrics, listening stats, history.
- Local library: liked tracks and playlists mixing both services; Spotify library pages when signed in; import of a service playlist into a mixed one.
- Themes: a manifest of 18 color tokens (four of them tile colors and their ink), fonts and a corner radius, a public contract for theme authors: the app draws everything with them. Ten built-in themes: Mix (default), Daylight, Midnight, Sakura, Terminal, Aurora, Greenroom, Glasshouse, Anime, Mochi; eight add effects the app draws (Mix's tile textures, Aurora's drifting lights and frosted glass, Sakura's petals, Terminal's scanlines and glow, Greenroom's and Glasshouse's grounds in the colour of the cover playing, Anime's speed lines and screentone, Mochi's rising sparkles), tuned with options (toggle, select, color, range). Mochi's round face, Quicksand, ships with the app beside Selawik.
- Plugins: ES modules given a plugin API (player, queue, catalog, library, commands, settings, storage, and pages, widgets and track actions made of elements and canvases the app draws), each in a JavaScript engine of the app's own. Every plugin can use the whole API, whatever its permissions say; the app tells users so before enabling a plugin.
- Community registry: themes and plugins live in the repository's `community/` folder and are installed from it over HTTPS.
- macOS, Windows and Linux are the supported platforms.

## Brand Commitments

- Name: MixMusic. The icon is two overlapping rings with a play triangle where they meet, on a violet-to-coral gradient (`resources/icon.png`, drawn by `tools/genicon`).
- The project never names other projects as references or inspirations in its code, docs or interface.
- Spotify and YouTube Music are named as the services MixMusic plays, never imitated: no use of their logos or brand layouts. Each service has a theme token (`spotify`, `youtube`) used only to tag where a track comes from.

## Evidence on Hand

No testimonials, user counts, benchmarks or press exist. Do not invent any.

## Product Principles

1. One queue, two services: where a track comes from is visible but never in the way.
2. The player is a canvas: every surface is themeable through public tokens and hooks, and nothing a theme might target is styled through private, unstable selectors.
3. Fast for hands on the keyboard: everything common is one shortcut away, and the command palette reaches the rest.
4. Honest about limits: when Spotify or YouTube restricts something, the interface says what and why, and offers the way around it.
5. Lives quietly in the background: playing for hours costs little, and the tray, media keys and mini player do the everyday work.

## Accessibility & Inclusion

Keyboard operation of every control, visible focus, screen reader labels on icon buttons, and `prefers-reduced-motion` respected by the app and the built-in themes.
