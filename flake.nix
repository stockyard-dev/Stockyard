{
  description = "Stockyard — 150 self-hosted developer tools";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }: let
    supportedSystems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" ];
    forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
  in {
    packages = forAllSystems (system: let
      pkgs = nixpkgs.legacyPackages.${system};
      
      mkStockyardTool = { name, slug, version ? "1.0.0", description }: pkgs.stdenv.mkDerivation {
        pname = "stockyard-${slug}";
        inherit version;
        
        src = pkgs.fetchurl {
          url = "https://github.com/stockyard-dev/stockyard-${slug}/releases/download/v${version}/stockyard-${slug}_${version}_${if system == "x86_64-linux" then "linux_amd64" else if system == "aarch64-linux" then "linux_arm64" else "darwin_arm64"}.tar.gz";
          sha256 = pkgs.lib.fakeSha256;
        };
        
        sourceRoot = ".";
        installPhase = ''
          mkdir -p $out/bin
          cp stockyard-${slug} $out/bin/
        '';
        
        meta = with pkgs.lib; {
          inherit description;
          homepage = "https://stockyard.dev/${slug}/";
          license = licenses.asl20;
          platforms = platforms.linux ++ platforms.darwin;
        };
      };
    in {
      default = mkStockyardTool { name = "stockyard"; slug = "stockyard"; version = "1.1.0"; description = "The LLM infrastructure platform"; };
      bounty = mkStockyardTool { name = "bounty"; slug = "bounty"; description = "Self-hosted issue tracker"; };
      trough = mkStockyardTool { name = "trough"; slug = "trough"; description = "Self-hosted LLM cost tracker"; };
      saltlick = mkStockyardTool { name = "saltlick"; slug = "saltlick"; description = "Self-hosted feature flags"; };
      bellwether = mkStockyardTool { name = "bellwether"; slug = "bellwether"; description = "Self-hosted uptime monitor"; };
      headcount = mkStockyardTool { name = "headcount"; slug = "headcount"; description = "Self-hosted web analytics"; };
      strongbox = mkStockyardTool { name = "strongbox"; slug = "strongbox"; description = "Self-hosted secret manager"; };
      corral = mkStockyardTool { name = "corral"; slug = "corral"; description = "Self-hosted webhook relay"; };
      hub = mkStockyardTool { name = "hub"; slug = "hub"; description = "Stockyard tool manager"; };
      dossier = mkStockyardTool { name = "dossier"; slug = "dossier"; description = "Self-hosted CRM"; };
      cipher = mkStockyardTool { name = "cipher"; slug = "cipher"; description = "Self-hosted password manager"; };
      notebook = mkStockyardTool { name = "notebook"; slug = "notebook"; description = "Self-hosted notes"; };
      lasso = mkStockyardTool { name = "lasso"; slug = "lasso"; description = "Self-hosted link shortener"; };
      billfold = mkStockyardTool { name = "billfold"; slug = "billfold"; description = "Self-hosted invoicing"; };
    });
  };
}
