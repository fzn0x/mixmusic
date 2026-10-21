# Security

## Reporting a vulnerability

Please report security problems privately, through GitHub's
[private vulnerability reporting](https://github.com/fzn0x/mixmusic/security/advisories/new),
not in a public issue. Tell what an attacker could do, and how to reproduce
it. You will hear back within a week, and be credited in the fix unless you
prefer not to.

## What MixMusic protects

- **Your Spotify sign-in.** The token lives in the system's credential store,
  or in a file only your user can read where there is none. MixMusic never
  sees your password.
- **Its windows.** MixMusic draws its own windows. Its one web page,
  Spotify's player on Windows, may call one Go service, for the Spotify
  token that player needs; other pages, as Google's sign-in, call none.
  Links to websites open in your browser, not in the app's window.
- **Add-ons from the registry.** They are fetched over HTTPS, checked, size
  limited, and installed into a temporary folder that replaces the installed
  version only once valid. Theme manifests are checked, color by color, and
  add-on files stay inside their folder.
- **yt-dlp.** MixMusic downloads it from its official releases and checks it
  against the checksums the release publishes; it passes only validated
  video IDs to it.

## What it cannot protect

Plugins can use the whole plugin API, whatever their permissions say: they
control the player, read what you play, your settings and your mixes, like
tracks and make mixes, which reaches your accounts where you signed in,
talk to any server on the internet, and add to the interface. Each runs in a
JavaScript engine of its own, which gives it nothing more: not your files,
nor your sign-ins. MixMusic tells you so before enabling a plugin. Enable
plugins from authors you trust; plugins of the community registry are
reviewed before they are listed.
