- MixMusic starts again on Linux x64. 0.5.6's x64 downloads, the .deb, the
  .rpm, the AppImage and the archive, asked for a system loader that only
  NixOS has, so they didn't start on other systems. A copy that updated
  itself to 0.5.6 can't update on its own: install 0.5.7 from the release
  page, or run install.sh again. Your settings and library are kept (#166).
