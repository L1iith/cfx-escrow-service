FROM golang:1.24-bookworm AS builder

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cfx-escrow-service ./cmd/escrowd

FROM node:20-bookworm-slim

ENV HOME=/var/lib/cfx-escrow-service
ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium
ENV PUPPETEER_SKIP_DOWNLOAD=true

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates chromium git openssh-client \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/cfx-escrow-bot
COPY --from=uploader package.json package-lock.json ./
RUN npm ci --omit=dev && npm cache clean --force
COPY --from=uploader . .

RUN groupadd --gid 10001 escrow \
    && useradd --uid 10001 --gid 10001 --home-dir /var/lib/cfx-escrow-service --shell /usr/sbin/nologin escrow \
    && mkdir -p /var/lib/cfx-escrow-service \
    && chown -R escrow:escrow /var/lib/cfx-escrow-service /opt/cfx-escrow-bot

COPY --from=builder /out/cfx-escrow-service /usr/local/bin/cfx-escrow-service

USER escrow
WORKDIR /var/lib/cfx-escrow-service
ENTRYPOINT ["/usr/local/bin/cfx-escrow-service"]
