# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM node:22.20-bookworm-slim AS web-builder

WORKDIR /src/web

ENV YARN_CACHE_FOLDER=/usr/local/share/.cache/yarn

COPY web/package.json web/yarn.lock ./
RUN --mount=type=cache,target=/usr/local/share/.cache/yarn \
    corepack enable \
    && yarn config set registry https://registry.npmjs.org \
    && yarn install --frozen-lockfile --network-timeout 600000

COPY web/ ./

ARG APP_VERSION=dev
ENV DISABLE_ESLINT_PLUGIN=true
ENV VITE_APP_VERSION=${APP_VERSION}

RUN yarn build

FROM golang:1.25.0-bookworm AS tokilake-builder

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web-builder /src/web/build ./web/build

ARG APP_VERSION=dev

RUN go build -trimpath -ldflags "-s -w -X 'one-api/common/config.Version=${APP_VERSION}'" -o /out/tokilake .

FROM golang:1.25.0-bookworm AS tokiame-builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG APP_VERSION=dev

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X 'one-api/common/config.Version=${APP_VERSION}'" -o /out/tokiame ./cmd/tokiame

FROM debian:bookworm-slim AS runtime-base

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /etc/tokilake /root/.tokilake

WORKDIR /data

COPY config.example.yaml /etc/tokilake/config.example.yaml
COPY packaging/tokiame.json.example /etc/tokilake/tokiame.json.example

FROM runtime-base AS tokilake

COPY --from=tokilake-builder /out/tokilake /usr/local/bin/tokilake

EXPOSE 3000 19981

ENTRYPOINT ["/usr/local/bin/tokilake"]
CMD ["--config", "/data/config.yaml", "--log-dir", "/data/logs"]

FROM runtime-base AS tokiame

COPY --from=tokiame-builder /out/tokiame /usr/local/bin/tokiame

ENTRYPOINT ["/usr/local/bin/tokiame"]
