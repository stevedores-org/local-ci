{
  description = "local-ci - lightweight local CI runner";

  nixConfig = {
    extra-substituters = [ "https://nix-cache.stevedores.org" ];
    extra-trusted-public-keys = [ "oxidizedmlx-cache-1:uG3uzexkJno1b3b+dek7tHnHzr1p6MHxIoVTqnp/JBI=" ];
  };

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gnumake
            git
            attic-client
          ];
        };
      });
}
