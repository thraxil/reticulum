{
  description = "A development environment for cask";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs }: let
    system = "x86_64-linux";
    pkgs = nixpkgs.legacyPackages.${system};
  in {
    devShells.${system}.default = pkgs.mkShell {
      buildInputs = with pkgs; [
        pkgs.go_1_25
        golangci-lint
        gcc
        libcap
        python310
        nodejs
        vips
        pkg-config
      ];
      MY_ENVIRONMENT_VARIABLE = "world";
    };
  };
}
