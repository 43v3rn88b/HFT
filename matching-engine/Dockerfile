# Stage 1: The Builder
FROM rust:1.75-slim-bookworm as builder

# Install build dependencies (needed for rdkafka/C-bindings)
RUN apt-get update && apt-get install -y cmake g++ libssl-dev pkg-config

WORKDIR /usr/src/matching-engine
COPY . .

# Build the binary in release mode for maximum speed
RUN cargo build --release

# Stage 2: The Runner (The actual container that runs)
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y libssl3 ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /usr/src/matching-engine/target/release/matching-engine .

# Run the engine
CMD ["./matching-engine"]