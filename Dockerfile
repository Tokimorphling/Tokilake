FROM rust:alpine AS builder


RUN apk add --no-cache build-base openssl-dev pkgconfig libcrypto3 openssl-libs-static

RUN cargo install sqlx-cli --no-default-features --features postgres


WORKDIR /app

COPY . .
RUN export SQLX_OFFLINE=true
RUN rustup component add rustfmt
RUN cargo build --release


FROM alpine:3.21

RUN apk add --no-cache ca-certificates


RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app


COPY --from=builder /app/target/release/tokilake /app/tokilake


USER appuser

ENV RUST_LOG=info


EXPOSE 19981
EXPOSE 19982


CMD ["/app/tokilake"]