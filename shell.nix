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

    valkey        # Key-value store
    centrifugo    # Real-time communication
    minio         # Object storage (server)
    minio-client  # Object storage (mc CLI)

    # Useful dev tools
    goreman
    tmux
    lf
    tokei
    nix
    nixfmt-rfc-style
    ripgrep
    just
    neovim
    nodejs_24
    pnpm
    awscli2
  ];
  shellHook = ''
    # Root of the project
    export PROJECT_ROOT="$PWD"
    # Default environment variables
    export PORT=8080
    export GO_ENV=development
    export DATABASE_URL=mysql://root@localhost:3306/shipq
    export COOKIE_SECRET=supersecret
    # Fix bizzare problems with the nix deps fighting with the system ones.
    unset DEVELOPER_DIR
  '';
}
