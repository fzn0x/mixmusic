# Themes

A theme sets the colors, the typefaces and the corners of MixMusic, which
draws everything with them, and may set its layout: the shapes of its
controls, its density, the size of the covers and the style of the
headings and the player bar. A theme is a folder:

```
my-theme/
├── theme.json   the manifest: colors, fonts, radius, layout
└── preview.png  optional: an image for the marketplace
```

## Make one

1. In Settings › Appearance, choose **open themes folder**.
2. Copy a theme there, for example
   [community/themes/paper-mint](../community/themes/paper-mint), into a folder
   named after its new `id`.
3. Change `id`, `name` and `author`, then the colors.
4. Run **Reload add-ons** from the command palette (<kbd>Ctrl</kbd> <kbd>K</kbd>)
   and pick the theme in Settings › Appearance. Themes that fail to load are
   listed there, with the reason.

Every change to `theme.json` shows after a reload.

## theme.json

```json
{
  "id": "my-theme",
  "name": "My Theme",
  "version": "1.0.0",
  "author": "Your Name",
  "description": "What it looks like, in one sentence.",
  "scheme": "dark",
  "colors": {
    "bg": "#000000",
    "surface": "#0b0b0b",
    "surface-2": "#181818",
    "surface-3": "#272727",
    "border": "rgba(255, 255, 255, 0.16)",
    "text": "#ffffff",
    "text-muted": "#a3a3a3",
    "accent": "#00bde3",
    "accent-text": "#000000",
    "danger": "#ff5a4f",
    "success": "#a4e600",
    "youtube": "#ff4040",
    "spotify": "#1ed760",
    "tile-1": "#00bde3",
    "tile-2": "#e6007e",
    "tile-3": "#a4e600",
    "tile-4": "#ffb000",
    "tile-text": "#000000"
  },
  "fonts": {
    "body": "Selawik, 'Segoe UI', system-ui, sans-serif",
    "display": "Selawik, 'Segoe UI', system-ui, sans-serif"
  },
  "radius": "0px",
  "layout": {
    "shape": "pill",
    "controls": "tonal",
    "headings": "regular"
  },
  "files": ["theme.json"]
}
```

| Field | |
|---|---|
| `id` | lowercase letters, digits and dashes; the folder's name |
| `version` | a semantic version, as `1.2.0`; raise it with every change you publish |
| `scheme` | `dark` or `light`: the controls MixMusic draws, such as switches and sliders, follow it |
| `colors` | every color below, as `#rgb`, `#rrggbb`, `#rrggbbaa`, `rgb()`, `rgba()`, `hsl()`, `oklch()` or `transparent` |
| `fonts` | font-family lists, of which MixMusic uses the first the computer has: `body` for text, `display` for titles and the large lowercase words (the body's when left out) |
| `radius` | the corner radius of tiles and cards, as `12px`; smaller things round by 60% of it |
| `layout` | optional [layout tokens](#layout): shapes, density and the style of the parts |
| `preview` | an image of the theme, for the marketplace |
| `files` | every file of the theme: the registry installs exactly these |

### Colors

| Color | Used for |
|---|---|
| `bg` | the window's ground |
| `surface`, `surface-2`, `surface-3` | panels, then hovered and pressed things on them |
| `border` | hairlines between rows and panels |
| `text`, `text-muted` | ink, and secondary ink; outlined controls are drawn in `text` |
| `accent`, `accent-text` | everything live: the playing row, the selection, the active page, primary buttons, and ink on them |
| `danger`, `success` | errors and confirmations |
| `youtube`, `spotify` | the marks telling where a track comes from |
| `tile-1` … `tile-4`, `tile-text` | the flat tiles of the home page and the stats, and ink on them |

Keep text readable: `text-muted` on `bg`, `accent-text` on `accent` and
`tile-text` on every tile need a contrast of at least 4.5:1. When the user
picks an accent of their own, or lets it follow the artwork, MixMusic picks
the accent's ink itself, and keeps the accent readable on your `bg`.

### Fonts

MixMusic ships Selawik, Quicksand, Mochi's round face, Roboto Flex, the
Material themes' face, at weights 400, 500 and 700, and Archivo
SemiCondensed at 800, the Crate themes' headings'; other families must be
on the user's computer, so end each list with a family every system has:
`system-ui` or `sans-serif` for the system's font, `monospace` for its
monospaced one.

### Layout

The layout tokens make a design language of a theme, beside its colors:
how round its controls are, how dense its pages, how large its covers,
and how it sets its headings and its player bar. Each is optional: a
theme that leaves one out has Mix's, so a theme without `layout` looks as
themes did before them.

| Token | Values | Without it | Sets |
|---|---|---|---|
| `radius-small` | a length, up to `999px` | 60% of `radius` | the corners of rows, inputs, menus, icon buttons and artwork |
| `shape` | `box` or `pill` | `box` | buttons, chips, the search box and the rail's indicator: rounded by `radius-small`, or fully round |
| `row` | `40px` to `96px` | `52px` | the height of a row of tracks; from `60px`, rows show larger artwork |
| `gutter` | `8px` to `96px` | `40px` | the room at either side of the pages |
| `gap` | `0px` to `48px` | `16px` | the gap between covers; the home's tiles are half of it apart, and the sections of a page three times it |
| `rail` | `160px` to `360px` | `248px` | the width of the sidebar, unfolded |
| `cover` | `120px` to `320px` | `172px` | the size of covers on shelves; the head of an album or a playlist shows its artwork 21% larger |
| `player-height` | `64px` to `128px` | `92px` | the height of the player bar |
| `player` | `docked` or `floating` | `docked` | the player bar across the window's foot, or floating over it as a raised card rounded by `radius`; the side panel floats beside it then |
| `controls` | `outline` or `tonal` | `outline` | Mix's 2px outlines, flooded with the accent where chosen, or filled controls: the current page, the playing track and the chosen chip in a quieter fill, inked in the accent |
| `headings` | `light`, `regular` or `bold` | `light` | Mix's giant light lowercase words, or titles as written, at a regular weight and Material's sizes, or heavy; pivots become chips, and the interface's words take capitals |
| `cards` | `none` or `filled` | `none` | covers bare, or in filled cards, a step lighter while hovered; the settings' groups take cards too |
| `motion` | `standard`, `smooth` or `expressive` | `standard` | presses and hovers at once, or on Material's standard springs, which hardly overshoot, or on its expressive ones, which overshoot a little and morph the play button's shape |

Lengths are in `px`, or `rem` and `em` of 16 pixels. The compact density,
in Settings › Appearance, takes off a theme's `row`, `rail`,
`player-height` and `gutter` what it takes off Mix's, keeping rows at
`40px` at least. While the computer asks for less motion, the springs of
`smooth` and `expressive` jump to where they go.

With `controls` set to `tonal`, an accent the user picks, or one that
follows the artwork, reads at 4.5:1 on `surface`, as the text it sets on
the fills needs: keep yours so too. Material, Material Dark, Crate and
Crate Light set every token: copy one of them to start a design language
of your own.

## Effects

The themes MixMusic ships with add effects of their own, which MixMusic
draws and the user tunes in Settings › Appearance: Mix's tile textures,
Aurora's drifting lights and frosted glass, Sakura's petals, Terminal's
scanlines and glow, Greenroom's and Glasshouse's grounds in the colour of
the cover playing, Anime's speed lines and screentone, Mochi's rising
sparkles, and the Material themes' colours, which MixMusic makes of a
seed colour in Material's HCT color space, as Material's dynamic color
does: the cover's while it plays and "Follow the artwork" is on, the
accent you chose otherwise, or MixMusic's pink. A theme of yours sets the
tokens above, and MixMusic draws everything else with them.

Themes made for the web interface of MixMusic 0.4 and before still load:
MixMusic reads their colors, fonts and radius, and ignores their stylesheet
and their options.

## Share it

Themes in [community/themes](../community/themes) can be installed by everyone
from the marketplace. [community/README.md](../community/README.md) tells how
to add yours.
