FROM golang:1.25-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o pdf-service ./cmd/server

# Intermediate stage: install wkhtmltopdf and bundle its shared-library tree.
FROM debian:bookworm-slim AS wkhtmltopdf-stage
RUN apt-get update && apt-get install -y --no-install-recommends \
    wkhtmltopdf \
    ca-certificates \
    fontconfig \
    fonts-dejavu-core \
    && rm -rf /var/lib/apt/lists/*

# Pre-build the fontconfig cache so it is available in the distroless image.
RUN fc-cache -fv

# Collect wkhtmltopdf and every shared library it needs into /bundle.
RUN set -e; \
    mkdir -p /bundle/usr/local/bin; \
    cp /usr/bin/wkhtmltopdf /bundle/usr/local/bin/wkhtmltopdf; \
    chmod 755 /bundle/usr/local/bin/wkhtmltopdf; \
    ldd /usr/bin/wkhtmltopdf \
      | grep "=>" \
      | awk '{ print $3 }' \
      | grep "^/" \
      | sort -u \
      | while read lib; do \
          dir=$(dirname "$lib"); \
          mkdir -p "/bundle$dir"; \
          cp -L "$lib" "/bundle$lib"; \
        done

# Final distroless image — no shell, no package manager, no root user.
FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=builder /app/pdf-service .
COPY settings.yaml .

# wkhtmltopdf binary and its resolved shared-library tree
COPY --from=wkhtmltopdf-stage /bundle/ /

# Fonts and fontconfig (including pre-built cache)
COPY --from=wkhtmltopdf-stage /usr/share/fonts      /usr/share/fonts
COPY --from=wkhtmltopdf-stage /etc/fonts            /etc/fonts
COPY --from=wkhtmltopdf-stage /var/cache/fontconfig /var/cache/fontconfig

# CA certificates
COPY --from=wkhtmltopdf-stage /etc/ssl/certs/ca-certificates.crt \
                               /etc/ssl/certs/ca-certificates.crt

EXPOSE 8080
CMD ["/app/pdf-service"]
