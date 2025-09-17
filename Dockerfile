# ---- Build Stage ----
# Use the official Go image as the build environment
FROM golang:1.24-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source tree
COPY . .

# Build the application; -ldflags "-w -s" reduces the binary size
# CGO_ENABLED=0 ensures a static binary suitable for minimal images like Alpine
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/mcp-server-echart .

# ---- Final Stage ----
# Use a minimal base image
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/mcp-server-echart /app/mcp-server-echart

# Copy the template file and static assets directory
# Note: the static directory may be created at runtime, but template.html is required
COPY template.html ./

# Create the static directory and set safe permissions
RUN mkdir -p /app/static/charts && \
    chown -R nobody:nogroup /app && \
    chmod -R 755 /app
    
# Switch to a non-root user for better security
USER nobody:nogroup

# Expose the application port (default 8989)
# The PORT environment variable can override this at runtime
EXPOSE 8989

# Provide default environment variables
ENV PORT=8989
ENV LOG_LEVEL=info
ENV STATIC_DIR=/app/static
ENV PUBLIC_URL="http://localhost:8989"

# Run the application
CMD ["/app/mcp-server-echart"] 
