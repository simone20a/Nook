{
  description = "Nook: a terminal markdown note editor with a Focus Line";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      homeManagerModules.default = import ./home-module.nix self;
      homeModules.default = self.homeManagerModules.default;

      checks = forAllSystems (
        pkgs:
        let
          inherit (pkgs) lib;
          sample = lib.evalModules {
            specialArgs = { inherit pkgs; };
            modules = [
              self.homeModules.default
              {
                options.home.packages = lib.mkOption { type = lib.types.listOf lib.types.package; };
                options.xdg.configFile = lib.mkOption { type = lib.types.attrsOf lib.types.attrs; };
                config.programs.nook = {
                  enable = true;
                  settings = {
                    vim = true;
                    theme = "sample";
                  };
                  themes.sample.ui.banner = "fg=#abcdef bold";
                  themes.fromFile = ./themefile/themes/light.toml;
                };
              }
            ];
          };
        in
        {
          home-module = pkgs.linkFarm "nook-module-example" (
            lib.mapAttrsToList (name: file: {
              inherit name;
              path = file.source;
            }) sample.config.xdg.configFile
          );
        }
      );

      packages = forAllSystems (pkgs: {
        nook = pkgs.buildGoModule {
          pname = "nook";
          version = self.shortRev or self.dirtyShortRev or "dev";
          src = self;

          vendorHash = "sha256-GA0bnoXj/af54MMem3/6yleecvCdZSkM9TGsa/cJMdw=";
          postInstall = ''
            mv "$out/bin/Nook" "$out/bin/nook"
          '';

          ldflags = [
            "-s"
            "-w"
          ];

          meta = {
            description = "Terminal markdown note editor with a Focus Line";
            homepage = "https://github.com/Unisay/Nook";
            license = pkgs.lib.licenses.mit;
            mainProgram = "nook";
          };
        };
        default = self.packages.${pkgs.stdenv.hostPlatform.system}.nook;
      });

      apps = forAllSystems (pkgs: {
        default = {
          type = "app";
          program = pkgs.lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.nook;
        };
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            gotools
          ];
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
