# syntax=docker/dockerfile:1

# Build stage. The service resolves mapexGoKit through a local `replace
# ../mapexGoKit`, so the build context is the PARENT directory that holds both
# repos as siblings (see docker-compose.yml: context ../..).
FROM golang:1.25 AS build
WORKDIR /src
COPY mapexGoKit/ ./mapexGoKit/
COPY mapexMarketplace/ ./mapexMarketplace/
WORKDIR /src/mapexMarketplace
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/marketplace ./src

# Runtime stage: tiny static image — just the binary. The catalog (bundles) and
# the SQLite index come from volumes at runtime, so the image stays light and the
# catalog updates without rebuilding it.
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/marketplace /usr/local/bin/marketplace
ENV GO_ENV=production \
    HTTP_ADDRESS=0.0.0.0 \
    HTTP_PORT=6060 \
    CATALOG_DIR=/catalog \
    CATALOG_INDEX_PATH=/data/catalog-index.db \
    CORS_ORIGINS=*
EXPOSE 6060
ENTRYPOINT ["/usr/local/bin/marketplace"]
