FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y curl python3 procps && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/stockyard-demo
COPY seed.py /opt/stockyard-demo/seed.py
COPY start.sh /opt/stockyard-demo/start.sh
RUN chmod +x /opt/stockyard-demo/start.sh

# Download all tool binaries at build time
RUN mkdir -p bin data logs && \
    for tool in hub bounty corral seismograph paddock saltlick; do \
        echo "Downloading $tool..." && \
        URL=$(curl -fsSL "https://api.github.com/repos/stockyard-dev/stockyard-${tool}/releases/latest" | python3 -c "import sys,json;assets=json.load(sys.stdin).get('assets',[]);urls=[a['browser_download_url'] for a in assets if 'linux_amd64' in a['name']];print(urls[0] if urls else '')") && \
        if [ -n "$URL" ]; then \
            curl -fsSL "$URL" -o /tmp/${tool}.tar.gz && \
            tar xzf /tmp/${tool}.tar.gz -C bin/ && \
            mv bin/stockyard-${tool}* bin/stockyard-${tool} 2>/dev/null || true && \
            chmod +x bin/stockyard-${tool} && \
            rm /tmp/${tool}.tar.gz; \
        fi; \
    done

EXPOSE 9800

CMD ["/opt/stockyard-demo/start.sh"]
