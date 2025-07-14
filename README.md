# mcp-server-echart

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

一个基于 `mcp-go` 框架构建的 Go 服务，它提供了一个能动态生成 ECharts 图表页面的工具。

## ✨ 功能特性

- **动态图表生成**：通过调用工具，传入 ECharts 的 JSON 配置，即可生成一个独立的 HTML 图表页面。
- **高度可配置**：支持自定义图表的标题、宽度和高度。
- **现代化页面**：生成的图表页面拥有一个干净、现代化的外观。
- **Docker 支持**：提供 `Dockerfile`，可以轻松构建轻量、可移植的 Docker 镜像。
- **配置灵活**：通过环境变量或 `.env` 文件进行配置，轻松适配不同环境。

---

## 🚀 快速开始

### 1. 准备工作

- [Go](https://golang.org/) (版本 1.24 或更高)
- [Docker](https://www.docker.com/) (可选，用于容器化部署)

### 2. 克隆与配置

克隆本仓库到您的本地：
```bash
git clone https://github.com/cnkanwei/mcp-server-echart.git
cd mcp-server-echart
```

复制配置文件模板，并根据需要进行修改：
```bash
cp .env.example .env
```

`.env` 文件包含以下配置项：

- `PORT`: 服务监听的端口，同时提供MCP协议和静态文件服务 (默认: `8989`)
- `PUBLIC_URL`: 服务的公开访问URL (默认: `http://localhost:8989`)
- `LOG_LEVEL`: 日志级别 (例如 `info`, `debug`)
- `STATIC_DIR`: 生成的静态 HTML 文件存放的目录 (默认: `static`)

### 3. 安装依赖

```bash
go mod tidy
```

---

## 📦 如何运行

### 在本地运行

```bash
go run main.go
```

服务启动后，会在单一端口上提供服务：
- 端口：`.env` 文件中配置的 `PORT`（默认为 8989）
- MCP 协议：通过 `/mcp` 路径访问
- 静态文件：通过根路径 `/` 访问

### 使用 Docker 运行

1.  **构建 Docker 镜像:**
    ```bash
    docker build -t mcp-server-echart .
    ```

2.  **运行 Docker 容器:**
    ```bash
    # 基础运行
    docker run -p 8989:8989 -d --name my-echart-server mcp-server-echart

    # 自定义端口
    docker run -p 9090:9090 \
      -e PORT=9090 \
      -e PUBLIC_URL="http://localhost:9090" \
      -d --name my-echart-server mcp-server-echart
    ```

---

## 🛠️ 工具 (Tool) API

本服务提供了一个名为 `generate_echarts_page` 的工具。

### 参数

- `title` (string, **必需**): 图表页面的标题。
- `inputSchema` (object, **必需**): ECharts 的 JSON 配置对象。
- `width` (number, *可选*): 图表的宽度（像素），默认 800。
- `height` (number, *可选*): 图表的高度（像素），默认 600。

### 返回值

成功调用后，工具会返回一个 URL，指向新生成的图表页面。

---

## 💻 如何使用

本服务可以通过任何支持 StreamableHTTP 协议的 MCP 客户端进行调用。

### 客户端配置

如果您使用的 MCP 客户端支持通过配置文件连接到服务器，您可以添加如下配置来连接到本服务。

请将 `mcp-server-echart` 添加到您的客户端配置中，并将 URL 指向本服务的监听地址（默认为 `http://localhost:8989/mcp`）。

**示例配置 (`client-config.json`):**
```json
{
  "mcpServers": {
    "mcp-server-echart": {
      "url": "http://localhost:8989/mcp"
    }
  }
}
```

**另一个示例 (例如，用于浏览器环境):**
```json
{
  "mcpServers": {
    "browser-use-mcp-server": {
      "url": "http://localhost:8000/mcp"
    }
  }
}
```

> **注意**:
> - URL 应与您在 `.env` 文件中配置的 `PORT` 保持一致。
> - StreamableHTTP 协议的默认端点是 `/mcp`。
> - 服务现在在单一端口上同时提供 MCP 协议和静态文件服务。

### 客户端配置 (通过 Docker 命令)

如果您的 MCP 客户端支持通过命令启动服务，您也可以配置它来直接运行 Docker Hub 上的镜像。这会将服务作为一个子进程来管理，客户端通过标准输入/输出 (stdin/stdout) 与其通信。

这对于本地开发或将服务分发给他人使用非常方便。

**示例配置 (`client-config.json`):**
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
> **注意**:
> - `-p 8989:8989` 将容器的服务端口映射到主机，用于 MCP 协议通信和访问生成的图表页面。
> - `cnkanwei/mcp-server-echart:latest` 是发布在 Docker Hub 上的公共镜像。

---

## 📜 许可证

本项目基于 [MIT License](LICENSE) 开源。