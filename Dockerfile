FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git rsvg-convert

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=unknown

RUN rsvg-convert web/og-image.svg -o web/og-image.png && \
    rsvg-convert -w 192 -h 192 web/icon-192.svg -o web/icon-192.png && \
    rsvg-convert -w 512 -h 512 web/icon-512.svg -o web/icon-512.png
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
    -o /out/tagnote-server ./cmd/tagnote-server
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/tagnote-add      ./cmd/tagnote-add
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/tagnote-read     ./cmd/tagnote-read
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/tagnote-logs     ./cmd/tagnote-logs
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/tagnote-delete   ./cmd/tagnote-delete
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/tagnote-tags     ./cmd/tagnote-tags
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/tagnote-login    ./cmd/tagnote-login
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/tagnote-migrate  ./cmd/tagnote-migrate
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/tagnote-diagnose ./cmd/tagnote-diagnose

FROM alpine:3.19

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/ /usr/local/bin/

RUN adduser -D -u 1001 tagnote \
    && mkdir -p /data /data/uploads \
    && chown -R tagnote:tagnote /data

USER tagnote

EXPOSE 3000

ENTRYPOINT ["tagnote-server"]
CMD ["-addr", ":3000", "-db", "/data/tagnote.db", "-uploads", "/data/uploads"]
