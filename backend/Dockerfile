# ---------- build stage ----------
FROM golang:1.22-alpine AS build
WORKDIR /src

# Cache deps first for faster rebuilds
COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .
# modernc.org/sqlite is pure Go (CGO disabled), so we can produce a tiny static binary.
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags="-s -w" -o /out/makesense ./cmd/makesense

# ---------- runtime stage ----------
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/makesense /app/makesense

# Cloud Run sets $PORT. Defaults to 8080 in the app if unset.
ENV PORT=8080
EXPOSE 8080

USER nonroot
ENTRYPOINT ["/app/makesense"]
