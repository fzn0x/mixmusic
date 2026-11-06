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
  // The icon that `mygo build` makes each platform's icons of. It isn't
  // in resources/, MyGo's place for it by default: `mygo build` ships every
  // file there beside the app, which reads none as it runs (#303).
  icon: "brand/icon.png",
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
  // The release workflow signs with the Developer ID and the App Store
  // Connect key of its secrets, which it puts in a keychain of its own:
  // `mygo build` signs the app with the hardened runtime and a timestamp,
  // has Apple notarize the disk image, and staples the ticket to both.
  // MixMusic needs no entitlements: it isn't sandboxed, records nothing,
  // sends no Apple events and loads only the system's frameworks; its
  // plugins run in sobek, which makes no machine code, and its web pages in
  // WebKit's own processes. Without the secrets, as on a contributor's Mac,
  // the app is signed ad hoc.
  macos: {
    signingIdentity: process.env.MACOS_SIGNING_IDENTITY || undefined,
    notarize: process.env.NOTARY_KEYCHAIN
      ? { keychainProfile: "mixmusic-notary", keychain: process.env.NOTARY_KEYCHAIN }
      : undefined,
    // macOS asks the user before MixMusic finds cast devices on the network
    // and plays on them, with this reason.
    infoPlist: {
      NSLocalNetworkUsageDescription: "MixMusic finds the cast devices on your network, such as Chromecasts and Nest speakers, to play on them.",
      NSBonjourServices: ["_googlecast._tcp"],
    },
  },
  // The workflow's code signing certificate, whose password is
  // MYGO_WINDOWS_CERTIFICATE_PASSWORD: osslsigncode signs the app, the
  // installer and its uninstaller with it, timestamped. Unsigned without it.
  windows: {
    certificate: process.env.WINDOWS_CERTIFICATE_FILE || undefined,
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
