# mcp-server-echart

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A Go service built with the `mcp-go` framework that provides a tool for dynamically generating ECharts chart pages.

## ✨ Features

- **Dynamic chart generation**: Provide an ECharts JSON configuration to the tool to create a standalone HTML chart page.
- **Highly configurable**: Customize the chart title, width, and height.
- **Modern page layout**: Generated chart pages feature a clean, modern design.
- **Docker support**: Includes a `Dockerfile` for building lightweight, portable Docker images.
- **Flexible configuration**: Configure the service with environment variables or a `.env` file to fit different environments.

---

## 🚀 Quick start

### 1. Prerequisites

- [Go](https://golang.org/) (version 1.24 or higher)
- [Docker](https://www.docker.com/) (optional, for containerized deployments)

### 2. Clone and configure

Clone the repository:

```bash
git clone https://github.com/cnkanwei/mcp-server-echart.git
cd mcp-server-echart
```

Copy the configuration template and adjust it as needed:

```bash
cp .env.example .env
```

The `.env` file supports the following settings:

- `PORT`: The port for both MCP requests and static file hosting (default: `8989`).
- `PUBLIC_URL`: The public URL of the service (default: `http://localhost:8989`).
- `LOG_LEVEL`: Logging level (for example `info`, `debug`).
- `STATIC_DIR`: Directory for generated static HTML files (default: `static`).

### 3. Install dependencies

```bash
go mod tidy
```

---

## 📦 How to run

### Run locally

```bash
go run main.go
```

After the service starts it exposes everything on a single port:
- Port: `PORT` from the `.env` file (defaults to 8989).
- MCP protocol: Available at the `/mcp` path.
- Static files: Available from the root `/` path.

### Run with Docker

1.  **Build the Docker image:**
    ```bash
    docker build -t mcp-server-echart .
    ```

2.  **Run the Docker container:**
    ```bash
    # Basic usage
    docker run -p 8989:8989 -d --name my-echart-server mcp-server-echart

    # Custom port
    docker run -p 9090:9090 \
      -e PORT=9090 \
      -e PUBLIC_URL="http://localhost:9090" \
      -d --name my-echart-server mcp-server-echart
    ```

---

## 🛠️ Tool API

The service exposes a tool named `generate_echarts_page`.

### Parameters

- `title` (string, **required**): Title of the chart page.
- `inputSchema` (object, **required**): JSON configuration object for ECharts.
- `width` (number, *optional*): Chart width in pixels (default: 800).
- `height` (number, *optional*): Chart height in pixels (default: 600).

### Return value

On success the tool responds with a URL pointing to the generated chart page.

---

## 💻 Usage

Any MCP client that supports the StreamableHTTP protocol can call this service.

### Client configuration

If your MCP client connects to servers through a configuration file, add the following entry to reach this service.

Add `mcp-server-echart` to your client configuration and point the URL to the service address (default: `http://localhost:8989/mcp`).

**Example configuration (`client-config.json`):**

```json
{
  "mcpServers": {
    "mcp-server-echart": {
      "url": "http://localhost:8989/mcp"
    }
  }
}
```

**Another example (for instance, a browser-based environment):**

```json
{
  "mcpServers": {
    "browser-use-mcp-server": {
      "url": "http://localhost:8000/mcp"
    }
  }
}
```

> **Notes:**
> - The URL must match the `PORT` configured in your `.env` file.
> - The default endpoint for the StreamableHTTP protocol is `/mcp`.
> - The service hosts both the MCP endpoint and static files on the same port.

### Client configuration (via Docker command)

If your MCP client can launch services via commands, configure it to run the Docker Hub image directly. The client manages the service as a subprocess and communicates through standard input/output (stdin/stdout).

This workflow is convenient for local development or sharing the service with others.

**Example configuration (`client-config.json`):**

```json
{
  "mcpServers": {
    "mcp-server-echart-docker": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-p",
        "8989:8989",
        "-e",
        "PORT=8989",
        "-e",
        "PUBLIC_URL=http://localhost:8989",
        "cnkanwei/mcp-server-echart:latest"
      ]
    }
  }
}
```

> **Notes:**
> - `-p 8989:8989` maps the container port to the host for both MCP traffic and access to generated chart pages.
> - `cnkanwei/mcp-server-echart:latest` is the public image published on Docker Hub.

---

## 📜 License

This project is released under the [MIT License](LICENSE).
