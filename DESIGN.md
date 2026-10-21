---
name: MixMusic
description: One player for Spotify and YouTube Music, drawn as giant lowercase words and flat colour tiles on true black.
colors:
  accent-cyan: "#00bde3"
  accent-ink: "#000000"
  tile-cyan: "#00bde3"
  tile-magenta: "#e6007e"
  tile-lime: "#a4e600"
  tile-amber: "#ffb000"
  tile-ink: "#000000"
  ground-black: "#000000"
  surface-panel: "#0b0b0b"
  surface-raised: "#181818"
  surface-pressed: "#272727"
  hairline: "rgba(255, 255, 255, 0.16)"
  ink-white: "#ffffff"
  ink-muted: "#a3a3a3"
  danger-coral: "#ff5a4f"
  success-lime: "#a4e600"
  source-youtube: "#ff4040"
  source-spotify: "#1ed760"
typography:
  display:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "clamp(3.5rem, 14cqi, 9rem)"
    fontWeight: 300
    lineHeight: 0.86
    letterSpacing: "-0.045em"
  headline:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "clamp(2.25rem, 7cqi, 3.5rem)"
    fontWeight: 300
    lineHeight: 1
    letterSpacing: "-0.035em"
  pivot:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "clamp(1.75rem, 4cqi, 3rem)"
    fontWeight: 300
    lineHeight: 1.1
    letterSpacing: "-0.02em"
  title:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "2.125rem"
    fontWeight: 300
    lineHeight: 1.1
    letterSpacing: "-0.025em"
  tile-title:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "clamp(1.5rem, 3.4cqi, 2.375rem)"
    fontWeight: 300
    lineHeight: 0.95
    letterSpacing: "-0.035em"
  subtitle:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "1.375rem"
    fontWeight: 300
    lineHeight: 1.3
  body:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 400
    lineHeight: 1.45
  label:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 400
    lineHeight: 1.3
  caption:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: 1.3
  mono:
    fontFamily: "Cascadia Mono, Consolas, DejaVu Sans Mono, ui-monospace, monospace"
    fontSize: "0.8125rem"
    fontWeight: 400
    lineHeight: 1.5
  site-hero-word:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "clamp(7rem, 21vw, 21rem)"
    fontWeight: 300
    lineHeight: 0.8
    letterSpacing: "-0.045em"
  site-section-word:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "clamp(5.5rem, 13vw, 13rem)"
    fontWeight: 300
    lineHeight: 0.86
    letterSpacing: "-0.045em"
  site-hook:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "clamp(1.625rem, 2.3vw, 2.375rem)"
    fontWeight: 300
    lineHeight: 1.12
    letterSpacing: "-0.025em"
  site-lead:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "clamp(1.5rem, 2.3vw, 2.125rem)"
    fontWeight: 300
    lineHeight: 1.18
    letterSpacing: "-0.02em"
  site-body:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 400
    lineHeight: 1.5
  site-row-title:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 400
    lineHeight: 1.5
  site-row-line:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 400
    lineHeight: 1.5
  site-small:
    fontFamily: "Selawik, Segoe UI, Noto Sans, DejaVu Sans, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.4
rounded:
  square: "0px"
  round: "50%"
spacing:
  "1": "4px"
  "2": "8px"
  "3": "12px"
  "4": "16px"
  "5": "24px"
  "6": "32px"
  "7": "48px"
  "8": "64px"
  row: "52px"
  page-gutter: "40px"
  site-gutter: "clamp(20px, 3.4vw, 56px)"
  site-section-gap: "clamp(112px, 12vw, 200px)"
components:
  button-primary:
    backgroundColor: "{colors.accent-cyan}"
    textColor: "{colors.accent-ink}"
    typography: "{typography.body}"
    rounded: "{rounded.square}"
    padding: "0 24px"
    height: "40px"
  button-outline:
    backgroundColor: "transparent"
    textColor: "{colors.ink-white}"
    typography: "{typography.body}"
    rounded: "{rounded.square}"
    padding: "0 24px"
    height: "40px"
  button-outline-active:
    backgroundColor: "{colors.ink-white}"
    textColor: "{colors.ground-black}"
  transport-round:
    backgroundColor: "transparent"
    textColor: "{colors.ink-white}"
    rounded: "{rounded.round}"
    size: "38px"
  transport-round-main:
    backgroundColor: "{colors.ink-white}"
    textColor: "{colors.ground-black}"
    rounded: "{rounded.round}"
    size: "48px"
  transport-round-main-hover:
    backgroundColor: "{colors.accent-cyan}"
    textColor: "{colors.accent-ink}"
  nav-box:
    backgroundColor: "transparent"
    textColor: "{colors.ink-white}"
    rounded: "{rounded.square}"
    size: "42px"
  nav-box-current:
    backgroundColor: "{colors.accent-cyan}"
    textColor: "{colors.accent-ink}"
  chip:
    backgroundColor: "transparent"
    textColor: "{colors.ink-white}"
    typography: "{typography.label}"
    rounded: "{rounded.square}"
    padding: "0 16px"
    height: "34px"
  chip-checked:
    backgroundColor: "{colors.accent-cyan}"
    textColor: "{colors.accent-ink}"
  input:
    backgroundColor: "{colors.ground-black}"
    textColor: "{colors.ink-white}"
    typography: "{typography.body}"
    rounded: "{rounded.square}"
    padding: "0 12px"
    height: "40px"
  tile:
    backgroundColor: "{colors.tile-cyan}"
    textColor: "{colors.tile-ink}"
    typography: "{typography.tile-title}"
    rounded: "{rounded.square}"
    padding: "12px 16px"
    height: "148px"
  track-row:
    backgroundColor: "transparent"
    textColor: "{colors.ink-white}"
    typography: "{typography.body}"
    rounded: "{rounded.square}"
    padding: "4px 8px"
    height: "52px"
  track-row-playing:
    backgroundColor: "{colors.accent-cyan}"
    textColor: "{colors.accent-ink}"
  menu-item-hover:
    backgroundColor: "{colors.accent-cyan}"
    textColor: "{colors.accent-ink}"
    height: "36px"
  toast:
    backgroundColor: "{colors.ink-white}"
    textColor: "{colors.ground-black}"
    typography: "{typography.label}"
    rounded: "{rounded.square}"
    padding: "12px 12px 12px 16px"
  site-button-large:
    backgroundColor: "{colors.accent-cyan}"
    textColor: "{colors.accent-ink}"
    typography: "{typography.site-body}"
    rounded: "{rounded.square}"
    padding: "0 28px"
    height: "52px"
  site-upnext-row:
    backgroundColor: "transparent"
    textColor: "{colors.ink-white}"
    typography: "{typography.body}"
    rounded: "{rounded.square}"
    padding: "6px 10px"
  site-upnext-row-playing:
    backgroundColor: "{colors.accent-cyan}"
    textColor: "{colors.accent-ink}"
  site-player-bar:
    backgroundColor: "{colors.surface-panel}"
    textColor: "{colors.ink-white}"
    height: "92px"
  site-download-tile:
    backgroundColor: "{colors.tile-cyan}"
    textColor: "{colors.tile-ink}"
    typography: "{typography.tile-title}"
    rounded: "{rounded.square}"
    padding: "18px 20px 20px"
    height: "300px"
  site-theme-card:
    backgroundColor: "transparent"
    textColor: "{colors.ink-white}"
    rounded: "{rounded.square}"
    padding: "16px"
    height: "132px"
---

# Design System: MixMusic

## Overview

**Creative North Star: "The Typographic Tile Wall"**

MixMusic is a wall of type and colour on true black. Every place in the player names itself in one giant, light, lowercase word that fills the width of the pane and is never cut; beneath it, a row of pivot words names the page's sections. Music sits on flat fields of colour (cyan, magenta, lime, amber) and on square artwork, never on rounded cards floating over charcoal. The interface is drawn almost entirely in white ink at light weights, with a single live colour that floods whatever is current.

Density is that of a tool kept open for hours: a 248px rail of outlined icon boxes, a main pane of giant words, tiles and shelves, a 340px queue column that always shows what plays next, and a 92px player bar. The pane is scrolled, the shell never is. Motion is short and directional: pages swing in from the right like a turnstile, pressed tiles tilt toward the pointer, the live tile in the rail flips over to show the next track.

The system is a public contract. Themes set its colours, faces and radius, the tokens `docs/themes.md` lists, and the app draws everything else from them. The default theme (Mix) is the reference expression; Daylight, Midnight, Sakura, Terminal, Aurora, Greenroom, Glasshouse, Anime and Mochi are token variants of the same grammar (light ground, other accents, other radii, other faces), not separate designs. The effects a built-in theme adds are its own, not house style: they live on the ground under the content (Aurora's lights, Sakura's petals, Greenroom's and Glasshouse's cover colours, Anime's speed lines and screentone, Mochi's sparkles), in the frosted glass of dialogs (Aurora, Glasshouse), in the page titles (Sakura's italics, Terminal's glow) or over the window (Terminal's scanlines), while tiles, rows and controls keep the flat grammar.

**Key Characteristics:**
- Giant lowercase light-weight words as the primary navigation landmark, shrunk to fit the pane, never cropped.
- Flat colour tiles with ink in `tile-text`; texture printed only inside tiles.
- One live colour (the accent, or the artwork's colour when dynamic) for every current, selected or playing thing, applied as a flood.
- Secondary controls are 2px white outlines; pressing inverts them to solid ink.
- Square corners by default; circles only for transport, artist portraits and stats.
- Where a track comes from is a mark with a shape, not just a colour.

## Colors

A true-black ground, white ink, four saturated tile colours and one accent that does all the live work.

### Primary
- **Live Cyan** (accent-cyan): the single colour of everything live: the current nav box, the playing track row and queue row, the hovered menu item, the selected palette row, checked chips, segments and switches, the progress and volume fill, the focus ring, text selection, the active lyric line, side-list headings and counts. When the user lets the accent follow the artwork, the app replaces it with a colour drawn from the cover (the captures show an ochre) and picks its ink.
- **Accent Ink** (accent-ink): the text and icons on anything flooded with the accent.

### Secondary
- **Tile Cyan, Tile Magenta, Tile Lime, Tile Amber** (tile-cyan, tile-magenta, tile-lime, tile-amber): the four tones of flat tiles (`data-tone` 1 to 4), the stats circles and plugin previews in the marketplace. Tones rotate; no tone means anything by itself.
- **Tile Ink** (tile-ink): black text on every tile. The Mix theme offers a "white text on tiles" option that switches it to white.

### Tertiary
- **Source Red** (source-youtube) and **Source Green** (source-spotify): only for the source marks and the service share in stats. They never colour a button, a heading or a surface.
- **Danger Coral** (danger-coral): errors, destructive menu items, the danger button, error toasts and problem panels (tinted 12% into the ground).
- **Success Lime** (success-lime): confirmations.

### Neutral
- **Ground Black** (ground-black): the window, the rail, the main pane, input fields.
- **Panel Black** (surface-panel): the queue column, the player bar, dialogs and the command palette.
- **Raised Grey** (surface-raised): menus, empty artwork frames, skeletons, code.
- **Pressed Grey** (surface-pressed): the unfilled part of sliders and scrollbar thumbs.
- **Hairline** (hairline): 1px dividers between panels, rows of settings, menu separators.
- **White Ink** (ink-white): all primary text, outlines (at 85%), the main transport button and toasts.
- **Muted Ink** (ink-muted): artists, albums, durations, hints, placeholders.

Derived tints, computed from the above so every theme inherits them: hover is ink at 7%, press ink at 13%, the section rule the accent at 45%, the dialog scrim the ground at 72%, and the search suggestions are raised grey laid over the ground, solid even where a theme's surfaces show the page through. Inactive pivots are ink at 38%, hovered pivots 70%.

### Named Rules
**The One Live Colour Rule.** Only the accent marks what is current, selected or playing, and it marks it by flooding the whole row, box or chip with the accent and switching the ink to accent ink. No second highlight colour, no tinted half-states for selection.

**The Source Tag Rule.** The service colours exist only to tag where a track comes from. They never become an accent, a fill or a heading colour.

## Typography

**Display Font:** Selawik (with Segoe UI, Noto Sans, DejaVu Sans, system-ui), shipped with the app in weights 300, 350, 400, 600 and 700. Quicksand, Mochi's round face, ships beside it in weights 300, 400 and 700, under the OFL as Selawik is.
**Body Font:** the same family; a theme's display face defaults to its body face.
**Mono Font:** the system's monospaced face (Consolas, Menlo, DejaVu Sans Mono), only for addresses and license texts.

**Character:** one humanist sans carries the whole interface, with the hierarchy made by size and weight alone: enormous and light at the top, plain and regular at reading size. Large type is always light (300) and tightly tracked.

### Hierarchy
- **Display** (300, clamp(3.5rem, 14cqi, 9rem), 0.86): the page word. One per page, lowercase, on one line shrunk to fit the pane, down to 45% of its size; a name longer than that wraps, balanced, at 0.95.
- **Headline** (300, clamp(2.25rem, 7cqi, 3.5rem), 1): empty-state titles, set in the accent.
- **Pivot** (300, clamp(1.75rem, 4cqi, 3rem), 1.1): the page's section switches under the page word, 0.66em apart, on one line shrunk to fit the pane, down to 85%; past that the row wraps at full size.
- **Title** (300, 2.125rem, 1.1): section titles over shelves and lists, and dialog titles; the queue column's title is the smaller 1.75rem pivot token.
- **Tile title** (300, clamp(1.5rem, 3.4cqi, 2.375rem), 0.95; clamp(2rem, 5.6cqi, 3.75rem) on large tiles): lowercase words on tiles; a word too long for its tile breaks.
- **Subtitle** (300, 1.375rem, 1.3): now-playing artists, the stats sentence, choice and marketplace card names, the palette input, lyrics in the side panel (2rem on the lyrics page).
- **Body** (400, 0.9375rem, 1.45): track titles, nav labels (17px at weight 350, or at the regular weight of a theme's own body face), list items, buttons. Prose is capped at 56 to 72ch.
- **Label** (400, 0.8125rem): artists, albums, durations, captions on tiles, chips, field labels, hints.
- **Caption** (400, 0.75rem): times, shortcuts, palette groups, marketplace meta.

The scale is multiplied by the user's text size. Sizes in `cqi` follow the width of the main pane, not of the window, so they shrink as the rail and the side panel take room. Numbers that change in place (times, counts) use tabular figures.

### Named Rules
**The Lowercase Voice Rule.** Page words, pivots, tile titles, section titles and nav labels are lowercase. Names of tracks, artists, albums and services keep their own capitalisation.

**The Light At Size Rule.** Everything at subtitle size and above is weight 300 with negative tracking (from -0.02em at pivots to -0.045em at the page word). Weight 600 appears only on the action inside a toast.

### Marketing Site Ramp
The landing page in `site/` reads at a larger size than the app, because it is read once rather than used for hours. Same family, same weights, same rules:
- **Hero word** (300, clamp(7rem, 21vw, 21rem), 0.8): the single "mixmusic" that opens the page, cropped by the main column.
- **Section word** (300, clamp(5.5rem, 13vw, 13rem), 0.86): one lowercase word per section, cropped by the column and sliding left as it scrolls; it swings in 44px from the right the first time it arrives. Under a 720px viewport the hero and section words become clamp(6rem, 30vw, 9rem) and clamp(4.5rem, 24vw, 7rem).
- **Hook** (300, clamp(1.625rem, 2.3vw, 2.375rem), 1.12): the one line under the hero that says what the player is.
- **Lead** (300, clamp(1.5rem, 2.3vw, 2.125rem), 1.18): the opening sentence of each section, capped at 26ch (34ch when wide) and balanced.
- **Body** (400, 1.0625rem, 1.5): marketing prose, capped at 60ch and set in ink at 80%; 1rem under a 720px viewport.
- **Row title over row line** (400, 1.125rem over 0.9375rem in muted ink): feature rows, registry entries; the player bar's title is the row title.
- **Small** (400, 0.875rem): meta lines, notes, download captions and labels; screenshot captions, file sizes and the fine print use the app's label size (0.8125rem).

Pivots under the hero word follow the app's pivot at up to 3.25rem; under section words they step down to clamp(1.75rem, 2.6vw, 2.5rem). Subsections (such as "what it needs") use the title size, clamp(1.75rem, 2.2vw, 2.125rem), on the app's 1px accent rule.

**The Reading Size Rule.** On the site, reading copy steps up one size from the app (body 1.0625rem, leads at weight 300 from 1.5 to 2.125rem with 1.12 to 1.18 leading); everything at lead size and above stays weight 300 with negative tracking, as the Light At Size Rule says.

## Layout

A fixed shell grid: rail (248px, 224px compact, 76px collapsed) and a top bar of 48px across the top; the main pane, then the side panel (340px, 300px under 1100px wide) when the queue or lyrics are open; the player bar (92px, 80px compact) across the bottom. The shell clips; only the pane, the rail's library list and the panel scroll.

The page sits inside a 40px gutter (28px compact), and the page word and the pivot row keep inside it: whatever the window, the rail and the side panel leave, every word of the header shows. Shelves scroll horizontally out to the edge so the next item peeks. Under a 600px pane a setting's control goes under its text.

Spacing steps are 4, 8, 12, 16, 24, 32, 48 and 64px. Sections and shelves are 48px apart; the header leaves 32px; a section title sits on a 1px accent rule with 8px under it and 16px before content. Track rows are 52px (40px compact).

Tiles live on a four-column grid capped at 1040px with an 8px gap and rows of at least 150px; tiles span 1x1, 2x1 (wide), 1x2 (tall) or 2x2 (large), packed densely. Under a 640px pane the grid becomes two columns. Shelves are rows of 172px artwork tiles with 16px gaps, snapping at the start; grids fill 164px columns. Collection heads pair a 208px cover (128px under a 720px pane) with facts aligned to its foot. Now playing pairs a 260 to 420px sleeve with the info column and stacks under 1100px.

Density is a root attribute (`data-density`), the rail's state is another (`data-sidebar`); both change measures through the same variables.

### Marketing Site
The site keeps the app's shell as its layout: a top bar, a main column, a 340px "up next" column in panel black (300px under 1180px) and a fixed 92px player bar (72px under 720px). The main column sits in a fluid gutter (site-gutter); sections are separated by one generous gap (site-section-gap), never by bands or alternating backgrounds. Inside a section, copy and screenshot share an asymmetric split (5 to 7, flipped to 7 to 5 on alternate sections, 6 to 6 beside code). Tiles, download tiles and theme cards use the app's four-column grid with an 8px gap (10px between theme cards). Under 1040px the up next column goes away, splits stack and four-column grids become two; under 720px the downloads stack to one column.

## Elevation & Depth

The world is flat. Tiles, artwork, rows, buttons, the rail, the panel and the player bar have no shadow, bevel or gradient; depth is told by tone (ground, then panel black, then raised grey), by 1px hairlines between panels, and by the accent flood on what is current.

Only things that float over the interface cast a shadow, a soft dark one with no offset colour: menus and suggestion lists, and toasts. Dialogs and the command palette float on a 72% ground scrim instead.

### Shadow Vocabulary
- **Menu float** (`box-shadow: 0 12px 32px -8px rgba(0, 0, 0, 0.55)`): context menus, submenus and search suggestions.
- **Toast float** (`box-shadow: 0 10px 28px -10px rgba(0, 0, 0, 0.6)`): toasts above the player bar.
- **Drop line** (`box-shadow: inset 0 2px 0 accent`): the insertion line while dragging in the queue; a mark, not depth.

### Named Rules
**The Flat Tile Rule.** A tile is a flat field of one tile colour. No gradient fill, bevel, or shadow, ever. A tile may carry its theme's printed texture in the lower corner and, on the home page, a faint luminosity print of artwork faded out under its words, so the ink always sits on the tile's colour.

**The Floating Only Rule.** Shadows belong to things that float above the window (menus, toasts). Anything that sits in the layout stays flat.

## Shapes

Corners are a theme token: the radius (0px in Mix and Daylight) for tiles, artwork, dialogs, choice and marketplace cards, and 60% of it for buttons, inputs, chips, rows and small art. In the default theme everything is square.

Circles are the one other form, and they carry meaning: transport buttons, the floating play button on artwork, artist portraits, the stats circles (whose radius grows with the square root of plays, from 14 to 60px) and the overlapping rings of the service share.

Lines are either 2px (outlines of secondary controls, the progress and volume tracks, the mini player's progress bar) or 1px (hairlines and the accent rule). The selected artwork tile gets a 3px inset accent outline; the selected theme card a 3px accent outline offset 2px.

## Components

### Buttons
Plain and blunt: a filled accent block or a white outline, nothing in between.
- **Shape:** square in Mix (radius-sm), 40px tall, 24px side padding, body size.
- **Primary:** accent fill, accent ink; on hover the accent mixes 14% toward ink.
- **Outline:** 2px outline in ink at 85%, ink text; hover takes the 7% tint; pressed inverts to a solid ink block with ground-coloured text.
- **Danger:** coral fill with ground-coloured text.
- **Press:** every button scales to 0.97 while pressed; disabled drops to 45% opacity.
- **Icon buttons:** 32px, muted ink, ink on hover with the hover tint; accent when on.
- **Text buttons:** small accent text, underline on hover.

### Transport
- **Round:** 38px circles with a 2px outline; the main play/pause is 48px, solid ink with ground-coloured icon, turning to the accent on hover; pulses while buffering. A toggled round (shuffle, repeat) outlines and draws in the accent.
- **Progress:** a 2px line, the accent up to the position, pressed grey beyond; the thumb (8 by 18px ink bar) appears only on hover or focus. Times sit at either end in caption size.

### Chips and Segments
- **Style:** 34px tall, 2px outline, small type.
- **State:** checked chips and segments flood with the accent; switches are a 46 by 22px outlined track with a 12px square thumb that slides and turns to accent ink on an accent track.

### Cards / Containers
- **Choice cards:** 2px outline, 16px padding, subtitle-size light title; checked floods with the accent.
- **Marketplace and theme cards:** 1px hairline border, no fill; theme cards show four 22px swatches of the theme's tiles.
- **Problems:** coral tinted 12% into the ground, 1px coral border at 45%.
- **Dialogs and palette:** panel black, 1px hairline, 32px padding (dialogs), on the scrim.

### Inputs / Fields
- **Style:** 40px, 2px outline, ground-black fill, up to 420px wide; selects draw their own chevron from two hard-stop gradients.
- **Focus:** the outline turns to the accent; the caret is always the accent.
- **Search:** the top bar's 36px outlined box, muted until focused.

### Navigation
- **Rail:** 42px outlined icon boxes with lowercase 17px labels beside them. Hover tints the box; the current page floods the box with the accent and turns its label to the accent. Collapsed, only the boxes remain.
- **Library list:** accent heading and accent counts, body-size names that turn accent on hover.
- **Live tile:** a 76px square at the foot of the rail showing the playing artwork, flipping every 12 seconds to a tile-cyan face that names the next track; hidden on windows shorter than 720px; still under reduced motion.
- **Command palette:** 640px, opened from the top bar or the keyboard; a light subtitle-size accent input; the selected row floods with the accent.

### Page Header (signature)
The page's name as one giant lowercase word in display type, never cropped and never truncated with an ellipsis: it shrinks to fit the pane on one line, and a name too long to fit at 45% of the display size (a long search, album or playlist) wraps, balanced. Pivots below are lowercase light words; the selected one is full ink, the rest 38% ink, with an optional count in small accent superscript. The pivot row shrinks to fit too, a little, and past 85% wraps at its full size, so every section shows.

### Flat Tile
Lowercase light title at the top left, an optional small caption at the foot (two-thirds width), in tile ink on one of four tile colours. Pressing tilts it up to 5 degrees toward the pointer and scales it to 0.97. In the Mix theme a dot-matrix (or pinstripe, or slash) texture is printed in the lower-right 30% at 55% opacity.

### Artwork Tile
A square, flat frame of artwork with its title in body size and a subtitle line that starts with the source mark. Hovering reveals a 42px accent play circle in the lower right; the item that plays wears a 3px inset accent outline. Artists are circular.

### Track Row and Queue Row
52px rows on a grid of number or art, title and artists, source mark, like, duration and more. Hover takes the 7% tint and reveals like and more; the playing row floods with the accent and its secondary text drops to 78% accent ink. Equaliser bars mark what plays. The queue column always lists the current track under "now playing" followed by what comes next.

### Source Mark
A 10px shape before every track's artists: a filled disc for Spotify, a filled square for YouTube Music, two interlocking rings for the user's own mixes. Its colour is the service token, but the shape alone tells the source, so it reads in monochrome and on the accent flood (where it turns accent ink).

### Overlays
Menus are raised grey with a hairline and the menu float shadow; items are 36px and flood with the accent on hover or focus. Toasts are inverted (an ink block with ground-coloured text) above the player bar; error toasts are coral, success toasts take the accent.

### Marketing Site
The landing page in `site/` is the player playing its own story. These patterns belong to it and are built from the app's parts.
- **Up next column:** panel black, sticky beside the main column, titled "up next" at the 1.75rem pivot size with an accent "now playing" label, as in the app's queue column. One queue row per section: a 48px square thumb (a tile in the section's tone with its stroked icon, unprinted), a body-size title and a small muted line led by the two-rings mark. The section in view floods with the accent; sections above it drop to 74% opacity as played. The column ends on a muted line saying where the queue ends. On short windows the column steps down, as far as the whole queue needs to stay in view: 40px thumbs, then 34px with smaller type, then 28px rows without their lines.
- **Player bar (signature):** the app's player bar, panel black over a hairline, and it plays the page. Its thumb and title turn (a 28px turnstile, 520ms) to the section in view, its sub line names what is next, its 2px accent progress line is the scroll position (click or arrow keys to seek), the round transport steps through sections and plays them in turn, and its right end always holds the primary action: the cyan "download for <your system>" block beside an "other systems" outline.
- **Buttons:** the app's primary and outline buttons at 44px (52px large in the hero and the close, 36px small in the top bar), the outline drawn as a 2px inset rule in outline ink.
- **Download tiles:** flat tiles in the four tones, 300px tall: the lowercase system name as the tile title, a small caption, then one row per file (download icon, name, size) on 30% tile-ink rules. They carry no printed texture, so the file rows stay legible. The visitor's own system gets a small "for your system" tag in tile ink on the tile's tone.
- **Theme gallery:** a large screenshot that crossfades (260ms) to the chosen theme, over a four-column row of theme cards. Each card is filled with its theme's own ground and ink, with a 1px hairline, four 22px swatches and the theme name at 1.5rem, weight 300; the current card wears the 3px accent outline offset 2px. A theme that sets its own face shows its name in that face, as a specimen of the theme, not as a face of the site.
- **Screenshots:** real captures, full width of their column on panel black with a 1px hairline, captioned in small muted type. The hero's is the README's banner: the player's pages in Mochi, in perspective around now playing.
- **Walkthrough:** the one-minute recording of the player, full width under its section's lead, framed and captioned as a screenshot, with the browser's own controls, muted, from a still of the recording. The section's pivots are its chapters: each seeks the video to its page, and the pivot of the page playing turns to ink.
- **Next release mark:** what main has and the latest release lacks carries a small chip reading "next release", in the registry chip's 1px outline and ink, after the sentence or caption it qualifies (a whole new section says it once, under its lead). The page drops every mark when GitHub's latest release is newer than the version it was built with.

### Motion
Ease-out `cubic-bezier(0.16, 1, 0.3, 1)` with 120, 260 and 520ms steps; ease-in-out for loops. Pages enter as a turnstile: each child slides 44px in from the right, the title first and the rest 50 to 120ms behind. All durations become 0 under reduced motion, and the live tile and the equaliser bars stop.

## Do's and Don'ts

### Do:
- **Do** open every page with its name in one lowercase display word, weight 300, letting it run past the pane's right edge.
- **Do** mark every current, selected or playing thing by flooding it with the accent and switching to accent ink.
- **Do** draw secondary buttons, inputs, chips and nav boxes as 2px outlines in the outline ink, inverting to solid ink when pressed.
- **Do** put destinations on flat tiles in the four tile tones, with lowercase light titles in tile ink.
- **Do** tag every track with its source mark, a shape first and a colour second.
- **Do** take every colour, face and radius from the theme's tokens, and every size and measure from the scale here.
- **Do** keep the next track in view: the queue column, the player bar's "next" line and the live tile's back face.
- **Do** let the site's player bar play the page: its progress is the scroll position, its title the section in view, and its primary action is always the download.

### Don't:
- **Don't** fill a tile with a gradient, or give tiles, artwork, rows or panels a shadow or bevel; shadows are for menus and toasts only.
- **Don't** introduce a second highlight colour; the service colours tag sources and nothing else.
- **Don't** set display, pivot, title or tile type in a heavy weight, or capitalise page words, pivots and tile titles.
- **Don't** round corners with a literal value; corners come from the theme's radius, or 60% of it, and circles are reserved for transport, play buttons, artists and stats.
- **Don't** crop or ellipsis-truncate the page word or the pivots; they shrink to fit the pane, then wrap.
- **Don't** print tile textures outside tiles.
