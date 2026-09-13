# Web dashboard build.
FROM node:22-bookworm-slim AS web
RUN npm i -g pnpm@9
WORKDIR /build
COPY pnpm-workspace.yaml pnpm-lock.yaml ./
COPY apps/web/package.json apps/web/
COPY packages/shared/package.json packages/shared/
RUN pnpm install --frozen-lockfile
COPY apps/web ./apps/web
COPY packages/shared ./packages/shared
RUN pnpm --dir apps/web build

# Server build. Web output syncs into the go:embed dir, same as `make build`.
FROM golang:1.25-bookworm AS server
WORKDIR /build/server
COPY apps/server/go.mod apps/server/go.sum ./
RUN go mod download
COPY apps/server ./
RUN mkdir -p cmd/cron-agent/webdist
COPY --from=web /build/apps/web/dist/. cmd/cron-agent/webdist/
RUN CGO_ENABLED=0 go build -o /out/cron-agent ./cmd/cron-agent

# Runtime: node for the opencode CLI the runner shells out to.
FROM node:22-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates \
	&& rm -rf /var/lib/apt/lists/* \
	&& npm i -g opencode-ai \
	&& useradd -m -s /bin/sh cronagent \
	&& mkdir -p /data /app /home/cronagent/.local/share /home/cronagent/.local/state /home/cronagent/.cache \
	&& chown -R cronagent:cronagent /data /app /home/cronagent
COPY --from=server /out/cron-agent /usr/local/bin/cron-agent
ENV DATA_DIR=/data PORT=8080 HOST=0.0.0.0 HOME=/home/cronagent
WORKDIR /app
EXPOSE 8080
USER cronagent
HEALTHCHECK --interval=30s --timeout=5s CMD curl -sf http://127.0.0.1:8080/api/health || exit 1
CMD ["cron-agent"]
