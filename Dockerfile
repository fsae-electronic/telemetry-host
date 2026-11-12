# ── Stage 1: build the Svelte frontend ───────────────────────────────────────
FROM node:22-bookworm-slim AS frontend
WORKDIR /src/app/frontend
COPY app/frontend/package.json app/frontend/package-lock.json ./
RUN npm ci
COPY app/frontend/ ./
# index.html uses the app logo as favicon
COPY app/cmd/app/logo.png ../cmd/app/logo.png
# vite outputs to ../cmd/app/frontend (see vite.config.js)
RUN npm run build

# ── Stage 2: build the Go binary ─────────────────────────────────────────────
FROM golang:1.25-bookworm AS backend
WORKDIR /src/app
COPY app/go.mod app/go.sum ./
RUN go mod download
COPY app/ ./
COPY --from=frontend /src/app/cmd/app/frontend ./cmd/app/frontend
RUN CGO_ENABLED=0 go build -o telemetry-host ./cmd/app/

# ── Windows executable (export only) ─────────────────────────────────────────
# docker build --target windows-exe --output type=local,dest=app .
FROM backend AS windows-build
# Embed the icon and file details (cmd/app/logo.png) as a Windows resource;
# go build picks up the generated rsrc_windows_amd64.syso automatically.
RUN go install github.com/tc-hib/go-winres@v0.3.3
RUN cd cmd/app && go-winres simply --icon logo.png --arch amd64 \
      --product-name "FSAE Telemetry" --file-description "FSAE Telemetry" \
      --file-version 1.0.0.0 --product-version 1.0.0.0
# -H windowsgui: no console window; the UI opens in its own window (window_windows.go)
RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o telemetry-host.exe ./cmd/app/

FROM scratch AS windows-exe
COPY --from=windows-build /src/app/telemetry-host.exe /

# ── Stage 3: runtime (binary + test environment tools) ──────────────────────
FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends socat python3 python3-pip curl ca-certificates \
 && pip3 install --no-cache-dir --break-system-packages pyserial \
 && rm -rf /var/lib/apt/lists/*

# Keep the repo layout that scripts/test-env.sh expects
WORKDIR /opt/telemetry
COPY --from=backend /src/app/telemetry-host app/telemetry-host
COPY app/scripts/ app/scripts/
COPY utils/ utils/
# Strip Windows CRLF line endings in case the repo was checked out on Windows
RUN sed -i 's/\r$//' app/scripts/*.sh utils/*.py && chmod +x app/scripts/*.sh

# telemetry.db is created in the working directory → persist via volume
WORKDIR /data
VOLUME /data
EXPOSE 8080

# Default: full test environment (socat + mock transmitter + host).
# Override with `docker run ... telemetry-host /opt/telemetry/app/telemetry-host --no-open`
# to run only the host.
CMD ["bash", "/opt/telemetry/app/scripts/test-env.sh"]
