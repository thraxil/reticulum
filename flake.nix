{
  description = "A development environment for cask";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/25.05-pre";
  };

  outputs = { self, nixpkgs }: let
    system = "x86_64-linux";
    pkgs = nixpkgs.legacyPackages.${system};
  in {
    devShells.${system}.default = pkgs.mkShell {
      buildInputs = with pkgs; [
        go
        gcc
        libcap
        python310
      ];
      MY_ENVIRONMENT_VARIABLE = "world";
    };
  };
}
