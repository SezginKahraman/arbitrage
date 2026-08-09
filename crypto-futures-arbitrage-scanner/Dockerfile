FROM node:22-alpine AS web-builder

WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS go-builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=web-builder /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/scanner .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S scanner \
    && adduser -S -G scanner scanner \
    && mkdir -p /app/data \
    && chown -R scanner:scanner /app

WORKDIR /app
COPY --from=go-builder /out/scanner ./scanner
COPY --from=web-builder /src/web/dist ./web/dist

ENV PORT=8082 \
    SCANNER_DB_PATH=/app/data/scanner.db

USER scanner
VOLUME ["/app/data"]
EXPOSE 8082

CMD ["./scanner"]
