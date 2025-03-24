{
  description = "Development environment for crossplane-skyhook";
  
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };
  
  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      devShells = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              nodejs_22
              yarn
              jq
              kubectl
              kubernetes-helm
              kind
              k9s
            ];
            
            shellHook = ''
              # Set up local KUBECONFIG
              export KUBECONFIG="$PWD/.kubeconfig"
              
              # Create .kubeconfig if it doesn't exist
              if [ ! -f "$KUBECONFIG" ]; then
                echo "Creating empty .kubeconfig file"
                touch "$KUBECONFIG"
              fi
              
              echo "KUBECONFIG set to $KUBECONFIG"
            '';
          };
        }
      );
    };
}
