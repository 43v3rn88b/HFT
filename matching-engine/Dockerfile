# Stage 1: The Builder (High-performance Rust 1.85)
FROM rust:1.85-alpine as builder

# Set the working directory
WORKDIR /usr/src/matching-engine

# Block 1: Basic compilers (Cached separately)
RUN apk add --no-cache musl-dev gcc g++ make cmake pkgconfig

# Block 2: Network libraries (Cached separately)
RUN apk add --no-cache curl-dev cyrus-sasl-dev zlib-dev openssl-dev

# Block 3: Static archives (Cached separately)
RUN apk add --no-cache zlib-static curl-static cyrus-sasl-static

# Tell Cargo to retry downloads if the connection is choppy
ENV CARGO_NET_RETRY=10

# Copy your code into the container
COPY . .

# Build the binary in release mode (fixes the Edition 2024 error)
RUN cargo build --release

# Stage 2: The Runner (Ultra-small production image)
FROM alpine:latest

# Install the runtime libraries needed to run the Kafka consumer
RUN apk add --no-cache libsasl libcurl zlib ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /usr/src/matching-engine/target/release/HFT .

# Start the engine
CMD ["./HFT"]