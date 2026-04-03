# Deploy Stockyard on Coolify

1. In Coolify dashboard, click **New Resource** → **Docker Compose**
2. Paste this compose file:

```yaml
services:
  stockyard:
    image: ghcr.io/stockyard-dev/stockyard:latest
    ports:
      - "4200:4200"
    environment:
      PORT: 4200
      DATA_DIR: /data
    volumes:
      - stockyard-data:/data

volumes:
  stockyard-data:
```

3. Set the domain and deploy.

Dashboard: `http://your-domain:4200/ui`
Proxy: `http://your-domain:4200/v1`
