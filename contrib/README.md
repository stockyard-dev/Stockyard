# App Store Manifests

Pre-built manifests for deploying Stockyard on self-hosting platforms.

## Umbrel

Submit as a PR to [getumbrel/umbrel-apps](https://github.com/getumbrel/umbrel-apps):

```bash
cp -r contrib/umbrel/ umbrel-apps/stockyard/
```

## CasaOS

Submit as a PR to [IceWhaleTech/CasaOS-AppStore](https://github.com/IceWhaleTech/CasaOS-AppStore):

```bash
cp contrib/casaos/docker-compose.yml CasaOS-AppStore/Apps/Stockyard/docker-compose.yml
```

## Coolify

Import directly in Coolify dashboard: New Resource → Docker Compose → paste contents of `contrib/coolify/docker-compose.yml`.

## Other Platforms

Stockyard runs as a single Docker container:

```bash
docker run -d -p 4200:4200 -v stockyard-data:/data ghcr.io/stockyard-dev/stockyard:latest
```
