self:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.programs.nook;
  inherit (lib) types mkOption;
  scalar = types.either types.str types.bool;
  key =
    name:
    if builtins.match "[a-zA-Z0-9_-]+" name != null then
      name
    else
      throw "Invalid Nook key or section name `${name}`";
  lines =
    attrs:
    lib.concatStrings (
      lib.mapAttrsToList (name: value: "${key name} = ${builtins.toJSON value}\n") attrs
    );
  renderTheme =
    attrs:
    lines (lib.filterAttrs (_: v: !builtins.isAttrs v) attrs)
    + lib.concatStrings (
      lib.mapAttrsToList (name: section: "[${key name}]\n${lines section}") (
        lib.filterAttrs (_: v: builtins.isAttrs v) attrs
      )
    );
  validate =
    name: src: command:
    pkgs.runCommand name { } ''
      cp ${lib.escapeShellArg "${src}"} "$out"
      ${command}
    '';
  nook = lib.getExe cfg.package;
  themeFile =
    name: value:
    let
      file =
        if builtins.match "[^/]+" name == null then throw "Invalid Nook theme filename" else "${name}.toml";
      src = if builtins.isAttrs value then pkgs.writeText "theme.toml" (renderTheme value) else value;
    in
    lib.nameValuePair "Nook/themes/${file}" {
      source =
        validate "nook-${lib.strings.sanitizeDerivationName file}" src
          ''${nook} theme check "$out"'';
    };
in
{
  options.programs.nook = {
    enable = lib.mkEnableOption "Nook";
    package = mkOption {
      type = types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.nook;
      description = "Nook package used for the editor and configuration validation.";
    };
    settings = mkOption {
      type = types.attrsOf scalar;
      apply = lib.mapAttrs (
        name: value:
        if
          lib.elem name [
            "theme"
            "notes_dir"
          ]
          && !builtins.isString value
        then
          throw "programs.nook.settings.${name} requires a string"
        else
          value
      );
      default = { };
      description = "Settings written to Nook/config.toml.";
    };
    themes = mkOption {
      type = types.attrsOf (
        types.either types.path (types.attrsOf (types.either types.str (types.attrsOf types.str)))
      );
      default = { };
      description = "Custom theme files or attribute sets keyed by theme name.";
    };
  };
  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];
    xdg.configFile = lib.mapAttrs' themeFile cfg.themes // {
      "Nook/config.toml".source = validate "nook-config.toml" (pkgs.writeText "config.toml" (
        lines cfg.settings
      )) ''NOOK_CONFIG="$out" ${nook} config show'';
    };
  };
}
