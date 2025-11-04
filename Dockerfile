# Dockerfile for e2ecp relay server
# This is a runtime-only Dockerfile that uses a pre-built binary
FROM scratch

# Copy the pre-built binary
COPY e2ecp /e2ecp

# Expose the default port
EXPOSE 3001

# Set default environment variables (can be overridden at runtime)
ENV PORT=3001 \
    MAX_ROOMS=10 \
    MAX_ROOMS_PER_IP=2 \
    LOG_LEVEL=info

# Run the relay server
ENTRYPOINT ["/e2ecp", "serve"]
CMD ["--port", "3001", "--max-rooms", "10", "--max-rooms-per-ip", "2", "--log-level", "info"]
