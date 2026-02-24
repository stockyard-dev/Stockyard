{ lib, stdenv, fetchurl }:

stdenv.mkDerivation rec {
  pname = "stockyard";
  version = "0.1.0";

  src = fetchurl {
    url = "https://github.com/stockyard-dev/stockyard/releases/download/v${version}/stockyard_linux_amd64.tar.gz";
    sha256 = "0000000000000000000000000000000000000000000000000000";
  };

  installPhase = ''
    install -Dm755 stockyard $out/bin/stockyard
  '';

  meta = with lib; {
    description = "LLM infrastructure proxy — 125 tools in one binary";
    homepage = "https://stockyard.dev";
    license = licenses.mit;
  };
}
