# Spotify in MixMusic on each system

"In MixMusic, from Spotify" plays Spotify tracks with Spotify's Web
Playback SDK, in the hidden window of `public/spotify.html`
([its web view](webviews.md)). Spotify encrypts its audio, and the SDK
plays only where the web view has a decryption module (a CDM) it takes.
Here is what each system's web view gives it, as tried for #295 in
November 2026, and what playing there would take. MixMusic plays Spotify
itself on Windows only.

| System | Its web view | Spotify in MixMusic |
|---|---|---|
| Windows | WebView2: Chromium, with Widevine | plays; the default |
| macOS | WKWebView: WebKit, with FairPlay | possible: the SDK starts with FairPlay; not yet heard |
| Linux | WebKitGTK: WebKit, without EME | not in its web view; costly with an embedded Chromium |

## What the SDK asks for

- **Its frame.** The SDK plays in an iframe of
  `https://sdk.scdn.co/embedded/index.html` (`allow="encrypted-media;
  autoplay"`). It looks for Widevine (`com.widevine.alpha`), then
  PlayReady, then FairPlay (`com.apple.fps.1_0`), through EME or WebKit's
  older `WebKitMediaKeys`.
- **Where none answers**, it fails with `initialization_error` ("Failed
  to initialize player") at once, before it asks for a token.
- **Widevine at software robustness** (`SW_SECURE_CRYPTO`, level L3) is
  enough. That is what desktop Chrome has on Linux.
- **On macOS, Widevine needs a host signed for Google's Verified Media
  Path (VMP).** Spotify played about a second of each song, then stopped,
  in a castlabs Electron build with Widevine, until the build was signed
  ([castlabs/electron-releases#193](https://github.com/castlabs/electron-releases/issues/193)).
  FairPlay is the system's own and needs no signing.

## macOS

The probe ran on macOS 26.5 (Apple silicon), in the test build of 0.5.9 and
MyGo 0.7.3's WKWebView. It used a hidden window of `public/spotify.html`, at
`mygo://localhost`, with a script in every frame that noted what the SDK's
frame asked of EME. The SDK got a made-up token, so no account was touched.

What it found:

- **EME.** `requestMediaKeySystemAccess` grants `com.apple.fps` and
  `com.apple.fps.1_0`, for `cenc`, `sinf` and `skd` init data, and
  `createMediaKeys` works. ClearKey is granted too. Widevine and PlayReady
  aren't.
- **The SDK starts.**
  - Its frame is a secure context. It finds FairPlay
    (`WebKitMediaKeys.isTypeSupported("com.apple.fps.1_0", "audio/mp4")`),
    makes `WebKitMediaKeys("com.apple.fps.1_0")`, and fetches Spotify's
    FairPlay certificate
    (`spclient.wg.spotify.com/fairplay-license/v1/application-certificate`,
    200).
  - Then it asks for the token. The made-up one ends in
    `authentication_error`, not `initialization_error`.
  - The same happens from an `https://` page, and with the web view's own
    user agent, which names no Safari. WKWebView needs no other scheme,
    origin or setting.
- **Media Source Extensions** play AAC in MP4 (`mp4a.40.2`, `mp4a.40.5`)
  and Opus in WebM, which Spotify streams. They don't play FLAC in MP4.
- **The hidden window plays sound without a gesture**, with or without
  MyGo's `AutoplayAllow`. That was tested with digital silence.
  - Its page is hidden (`document.visibilityState`), unlike WebView2's.
  - WebKit slows a hidden page's timers to once a second unless
    `BackgroundThrottling` is `ThrottlingOff`.
- **Not tried: the device getting ready, and a track sounding.** Both need
  a Spotify sign-in, which the test Mac doesn't have.
  - Spotify's Web Player supports Safari, which plays with the same
    FairPlay.
  - Whether Spotify's licence server answers WKWebView as it answers
    Safari is what a sign-in would tell.

What it would take, if a signed-in check plays:

1. `spotifyPlaysHere` true on macOS too, in `internal/app/spotifyhere.go`
   and `settings_playback.go`.
2. `canPlay` in `public/spotify.html` takes FairPlay (`com.apple.fps`), as
   it takes Widevine and PlayReady.
3. The device's window gets `BackgroundThrottling: ThrottlingOff` on macOS,
   for the SDK's own timers. Its autoplay needs nothing.
4. Update the text of Settings › Playback, `docs/spotify.md` and
   `docs/spotify-playback.md`. Add tests in a new file and a changelog
   fragment.
   - Connect stays the default on macOS until a few Macs have played this
     way.
5. Check with a Spotify sign-in on a Mac:
   - the device gets ready;
   - a track plays at volume 0, its time moving;
   - the gapless list, a seek near the end and the hush behave as on
     Windows (#184, #187, #273).

Steps 1 to 4 take about a day, plus the check.

**The visualizer** would need more:

- **The process.** WKWebView plays media in WebKit's GPU process,
  `com.apple.WebKit.GPU`, one per launch of the app. While the hidden page
  played, it was the only process running output (Core Audio's process
  list), with a new PID at each launch.
- **The tap.** A Core Audio process tap (`AudioHardwareCreateProcessTap`,
  macOS 14.2 and later) of that PID would hear it.
  - It needs `NSAudioCaptureUsageDescription` in the app's Info.plist, and
    the user agrees at the first capture.
  - The PID is known only through WebKit's private
    `-[WKWebView _gpuProcessIdentifier]`, which MyGo would have to expose.
- **Unknown:** whether a tap hears audio that FairPlay decrypted.

That is a few days of work on its own. Until then, Spotify plays on macOS
without the visualizer, as on Spotify Connect.

**An embedded Chromium (CEF) with Widevine** is the fallback if FairPlay
fails, and it is costly:

- CEF's minimal build for macOS arm64 is 135 MB to download and 353 MB
  unpacked.
- Widevine there needs VMP signatures, files that a signing certificate
  from Google makes. They come with a Widevine agreement with Google, or
  through castlabs' commercial certification. castlabs' free signing
  service signs only its own Electron builds.

## Linux

The probe ran in GitHub Actions, on a branch of its own since deleted. It
used Python's GObject bindings, with WebKitGTK 4.1, as MixMusic does:

- Ubuntu 24.04's WebKitGTK 2.52.6 (`libwebkit2gtk-4.1-0`), as the .deb
  gets it;
- the GNOME 51 runtime's WebKitGTK 2.54.0, which the Flatpak runs on.

The page was at an `https://` origin, with the same script in every frame
and the same made-up token.

What it found:

- **No EME in either.** `navigator.requestMediaKeySystemAccess` and
  `WebKitMediaKeys` are undefined in the page and in the SDK's frame. That
  stays so with WebKitSettings' `enable-encrypted-media` on.
- **The SDK fails at once**, with `initialization_error` ("Failed to
  initialize player") within 150 ms, and logs "Platform does not support
  navigator.requestMediaKeySystemAccess".
- **Why.** WebKitGTK has EME only when built with `ENABLE_ENCRYPTED_MEDIA`.
  That is off by default, and on in none of the Debian, Ubuntu, Fedora or
  GNOME runtime builds.
  - Even built in, WebKit brings only ClearKey.
  - Widevine would come through Thunder (OpenCDM), with closed binaries
    that Google licenses to whoever builds the device.

So MixMusic can't play Spotify itself in its web view on Linux. What
remains:

- **An embedded Chromium (CEF) with Widevine.** This is possible but
  costly:
  - **Size.** CEF 156's minimal build for Linux x64 is 333 MB to download
    and 1.6 GB unpacked; its `libcef.so` comes unstripped.
  - **The CDM.** Google's licence forbids redistributing it. CEF downloads
    it at the first run, about 22 MB, through Chromium's component updater,
    as Chromium's own builds and Firefox do.
  - **Codecs.** CEF's own builds leave out AAC, which Spotify's streams
    use. It would take a CEF built with proprietary codecs, and their
    patent licensing.
  - **Spotify** accepts software Widevine (L3), which is all Chrome has on
    Linux. Linux has no VMP.
  - **Packages.** The .deb, .rpm and AppImage would carry CEF.
    - The Flatpak would download the CDM into the app's folder and show it
      to Chromium's sandbox, as zypak's `ZYPAK_EXPOSE_WIDEVINE_PATH` does.
    - Flathub accepts only content it may redistribute; the CDM isn't
      shipped, so it stays out.
  - **From Go.** CEF is a second web engine beside MyGo's WebKitGTK, with
    its own processes and message loop.
    - energy (`energye/energy`) is the maintained Go binding to CEF; the
      others aren't maintained, and none documents Widevine.
  - **Cost.** Weeks of work, then a Chromium update every month or so, for
    its security fixes.
- **Spotify Soloist**, Spotify's own headless client (August 2026), for
  Linux, Apple silicon Macs and Windows.
  - It shows among the Spotify Connect devices, so MixMusic's "On Spotify
    Connect" plays on it today, with no Spotify app open. MixMusic could
    help find and choose it.
  - Users download it from Spotify, which forbids redistributing it.
  - It starts with an API key of the user's own, made with a Premium
    account.
  - Its builds expire after 90 days.
  - MixMusic's visualizer can't hear it on Linux.
- **Unofficial clients** (librespot, and spotifyd and ncspot, built on
  it) speak Spotify's private protocol. They are a known option only, not
  a plan:
  - They are outside Spotify's Developer Terms, whose IV.2.a.ii forbids
    reverse-engineering the Spotify Platform.
  - They could put the users' accounts and the app's client ID at risk.
  - In August 2026 Spotify closed its private endpoints to the client IDs
    of other apps
    ([librespot-org/librespot#1737](https://github.com/librespot-org/librespot/issues/1737)).
