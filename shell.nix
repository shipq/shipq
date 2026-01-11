{
  pkgs ? import (builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/871b9fd269ff6246794583ce4ee1031e1da71895.tar.gz";
    sha256 = "sha256:1zn1lsafn62sz6azx6j735fh4vwwghj8cc9x91g5sx2nrg23ap9k";
  }) { },
}:

pkgs.mkShell {
  buildInputs = with pkgs; [
    # Our programming language
    go
    # All 3 databases
    mysql80
    postgresql_18
    sqlite
    # Useful dev tools
    goreman
    lf
    tokei
    nix
    nixfmt-rfc-style
    ripgrep
    just
  ];
  shellHook = ''
    # Root of the project
    export PROJECT_ROOT="$PWD"
    # Fix bizzare problems with the nix deps fighting with the system ones.
    unset DEVELOPER_DIR
  '';
}
