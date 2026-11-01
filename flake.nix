{
  description = "MixMusic: Spotify and YouTube Music in one customizable desktop player";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      inherit (nixpkgs) lib;
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      linux = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      each = systems: f: lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
      version = (lib.importJSON ./package.json).version;
    in
    {
      # MixMusic for Linux, built from source: `nix build`, or
      # `nix run github:fzn0x/mixmusic`.
      packages = each linux (pkgs: rec {
        mixmusic = pkgs.callPackage ./nix/package.nix { inherit version; };
        default = mixmusic;
      });

      # The tools releases are built and published with, at the versions
      # flake.lock pins: Go, Bun for `mygo build`, NSIS for the Windows
      # installers, and GoReleaser; and Zola, which builds the website.
      # release.yml, ci.yml and pages.yml run in it, as `nix develop` does
      # here.
      devShells = each systems (pkgs: {
        default = pkgs.mkShell {
          packages =
            with pkgs;
            [
              go_1_27
              bun
              goreleaser
              jq
              zip
              zola
            ]
            ++ lib.optionals stdenv.hostPlatform.isLinux [
              nsis
              binutils
            ];
        };
      });

      formatter = each systems (pkgs: pkgs.nixfmt);
    };
}
