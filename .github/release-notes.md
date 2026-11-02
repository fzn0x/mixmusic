### Get it

| System | Download |
| --- | --- |
| **macOS** 12 and later, Apple silicon and Intel | [`mixmusic-{version}-macos-universal.dmg`](https://github.com/{repo}/releases/download/{tag}/mixmusic-{version}-macos-universal.dmg) |
| **Windows** 10 and 11, x64 | [`mixmusic-{version}-windows-amd64-setup.exe`](https://github.com/{repo}/releases/download/{tag}/mixmusic-{version}-windows-amd64-setup.exe) |
| **Windows** 11 on Arm | [`mixmusic-{version}-windows-arm64-setup.exe`](https://github.com/{repo}/releases/download/{tag}/mixmusic-{version}-windows-arm64-setup.exe) |
| **Windows**, without installing | [`mixmusic-{version}-windows-amd64-portable.zip`](https://github.com/{repo}/releases/download/{tag}/mixmusic-{version}-windows-amd64-portable.zip), or [`-arm64-portable.zip`](https://github.com/{repo}/releases/download/{tag}/mixmusic-{version}-windows-arm64-portable.zip) on Arm |
| **Debian, Ubuntu**, x64 | [`mixmusic-{version}-linux-amd64.deb`](https://github.com/{repo}/releases/download/{tag}/mixmusic-{version}-linux-amd64.deb) |
| **Debian, Ubuntu**, Arm64 | [`mixmusic-{version}-linux-arm64.deb`](https://github.com/{repo}/releases/download/{tag}/mixmusic-{version}-linux-arm64.deb) |
| **Fedora, openSUSE**, x64 | [`mixmusic-{version}-linux-amd64.rpm`](https://github.com/{repo}/releases/download/{tag}/mixmusic-{version}-linux-amd64.rpm) |
| **Fedora, openSUSE**, Arm64 | [`mixmusic-{version}-linux-arm64.rpm`](https://github.com/{repo}/releases/download/{tag}/mixmusic-{version}-linux-arm64.rpm) |
| **Any Linux**, one file | [`mixmusic-{version}-linux-amd64.AppImage`](https://github.com/{repo}/releases/download/{tag}/mixmusic-{version}-linux-amd64.AppImage), or [`-arm64.AppImage`](https://github.com/{repo}/releases/download/{tag}/mixmusic-{version}-linux-arm64.AppImage) on Arm64: make it executable and open it |
| **Any Linux**, sandboxed, x64 | [`mixmusic-{version}-linux-amd64.flatpak`](https://github.com/{repo}/releases/download/{tag}/mixmusic-{version}-linux-amd64.flatpak): `flatpak install ./mixmusic-{version}-linux-amd64.flatpak`, which fetches the GNOME runtime from Flathub the first time |
| **Any Linux**, x64 or Arm64 | `mixmusic-{version}-linux-<arch>.tar.gz` and [`install.sh`](https://github.com/{repo}/releases/download/{tag}/install.sh) side by side, then `sh install.sh` |

**First launch.** The macOS app is not notarized yet: open it once from System Settings › Privacy & Security › Open Anyway, or run `xattr -dr com.apple.quarantine /Applications/MixMusic.app`. The Windows installer is not signed yet: if SmartScreen warns, choose More info › Run anyway.

**Playing music.** YouTube Music plays without anything else, apart from a few tracks, such as those made for children, which need yt-dlp: MixMusic installs it for you from Settings › Playback. Spotify needs an app of your own, which the [setup guide](https://github.com/{repo}/blob/{tag}/docs/spotify.md) walks through. The [README](https://github.com/{repo}/blob/{tag}/README.md#install) lists what each system needs.

Verify downloads with `SHA256SUMS`.
