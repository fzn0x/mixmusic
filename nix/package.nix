# MixMusic for Linux, built from source. MyGo needs no C toolchain: it
# loads GTK, WebKitGTK, the tray's AppIndicator, the keyring and the sound
# libraries as it runs, which the wrapper finds for it.
{
  lib,
  buildGoModule,
  go_1_27,
  wrapGAppsHook3,
  copyDesktopItems,
  makeDesktopItem,
  gtk3,
  glib,
  glib-networking,
  gsettings-desktop-schemas,
  cairo,
  pango,
  at-spi2-core,
  gdk-pixbuf,
  libepoxy,
  webkitgtk_4_1,
  libsoup_3,
  libayatana-appindicator,
  libdbusmenu-gtk3,
  gtk-layer-shell,
  libsecret,
  libpulseaudio,
  alsa-lib,
  version,
}:

let
  # What MixMusic loads as it runs, by these libraries' names.
  runtimeLibraries = [
    gtk3
    glib
    cairo
    pango
    at-spi2-core
    gdk-pixbuf
    libepoxy
    webkitgtk_4_1
    libsoup_3
    libayatana-appindicator
    libdbusmenu-gtk3
    gtk-layer-shell
    libsecret
    libpulseaudio
    alsa-lib
  ];
in
(buildGoModule.override { go = go_1_27; }) {
  pname = "mixmusic";
  inherit version;

  src = lib.cleanSource ../.;
  # The hash of the Go modules go.sum names: a change of go.sum changes it,
  # and `nix build` then tells the new one.
  vendorHash = "sha256-Kgn/a+vNes144RQ4ySW/71jSaTf5BYOa132ca7NISow=";

  subPackages = [ "." ];
  env.CGO_ENABLED = "0";
  ldflags = [
    "-s"
    "-w"
  ];
  # CI runs the tests; some of them need the network or a sound card.
  doCheck = false;

  nativeBuildInputs = [
    wrapGAppsHook3
    copyDesktopItems
  ];
  # For the wrapper: GSettings' schemas, and TLS for WebKitGTK's sign-in.
  buildInputs = runtimeLibraries ++ [
    glib-networking
    gsettings-desktop-schemas
  ];

  preFixup = ''
    gappsWrapperArgs+=(--prefix LD_LIBRARY_PATH : "${lib.makeLibraryPath runtimeLibraries}")
  '';

  postInstall = ''
    install -Dm644 resources/icon.png $out/share/pixmaps/mixmusic.png
  '';

  desktopItems = [
    (makeDesktopItem {
      name = "mixmusic";
      desktopName = "MixMusic";
      comment = "Spotify and YouTube Music in one customizable player";
      exec = "mixmusic %U";
      icon = "mixmusic";
      categories = [
        "AudioVideo"
        "Audio"
        "Player"
      ];
      mimeTypes = [ "x-scheme-handler/mixmusic" ];
    })
  ];

  meta = {
    description = "Spotify and YouTube Music in one customizable desktop player";
    homepage = "https://github.com/fzn0x/mixmusic";
    license = with lib.licenses; [
      mit
      gpl3Only
    ];
    mainProgram = "mixmusic";
    platforms = lib.platforms.linux;
  };
}
