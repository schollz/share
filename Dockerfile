# Dockerfile for e2ecp relay server
# This is a runtime-only Dockerfile that uses a pre-built binary
FROM scratch

# Copy the pre-built binary
COPY e2ecp /e2ecp

# Expose the default port
EXPOSE 3001

# Run the relay server with default settings
# Override with: docker run -p PORT:PORT e2ecp-relay --port PORT [other flags]
ENTRYPOINT ["/e2ecp", "serve"]
CMD ["--port", "3001", "--max-rooms", "10", "--max-rooms-per-ip", "2", "--log-level", "info"]
