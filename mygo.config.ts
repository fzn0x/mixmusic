import { defineConfig } from "mygo-cli";
import pkg from "./package.json" with { type: "json" };

export default defineConfig({
  name: "MixMusic",
  // The slug names the update archives, the Windows installer and the disk
  // image, which release.yml and .github/scripts/package.sh look for by
  // name. It is MyGo's default for "MixMusic": pinned so it can't move.
  slug: "mixmusic",
  identifier: "app.mixmusic.desktop",
  version: pkg.version,
  copyright: "© 2026 MixMusic contributors",
  // MyGo draws the windows itself, in Go (the root package): the only pages
  // are that of MixMusic's own Spotify device, public/spotify.html, and that
  // of the picture of music videos, public/video.html with video.js, which
  // `mygo build` embeds as they are. They call Go without a generated
  // client, so `mygo dev` writes it out of the way.
  frontendDist: "public",
  bindings: ".mygo/bindings.ts",
  out: "build",
  urlSchemes: ["mixmusic"],
  // Apps update themselves from the releases of the repository, signed with
  // the key whose public half is here (the secret one is the
  // MYGO_UPDATER_PRIVATE_KEY secret of the release workflow).
  updates: {
    publicKey: "y/g7m+MircfXqdGupUFNygznyyR/Y41D1YAOYw41ISU=",
    github: "fzn0x/mixmusic",
  },
  linux: {
    comment: "Spotify and YouTube Music in one customizable player",
    categories: ["AudioVideo", "Audio", "Player"],
    command: "mixmusic",
    // MixMusic plays through PulseAudio, which PipeWire serves too, or
    // ALSA. The tray icon needs AppIndicator, and keeping sign-ins in the
    // keyring libsecret. WebKitGTK, which MyGo depends on, shows Google's
    // sign-in to YouTube Music.
    depends: ["libpulse0 | libasound2 | libasound2t64", "libayatana-appindicator3-1", "libsecret-1-0"],
  },
});
