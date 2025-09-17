package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

//go:embed template.html
var htmlTemplate string

// PageData holds data for the HTML template
type PageData struct {
	Title     string
	Width     int
	Height    int
	Option    template.JS
	OptionStr string
}

func init() {
	// Load environment variables from .env if present
	if err := godotenv.Load(); err != nil {
		log.Info("No .env file found; using defaults or system environment variables")
	}

	// Initialize logging
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	level, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)
}

func main() {
	// Create the MCP server
	s := server.NewMCPServer(
		"ECharts Visualization Page Service", // MCP service for generating ECharts pages
		"1.0.0",                              // Version number
	)

	// Register the generate_echarts_page tool
	generateEchartsPage := mcp.NewTool("generate_echarts_page",
		mcp.WithDescription("Generate an HTML chart page from an ECharts JSON configuration"),
		mcp.WithObject("inputSchema",
			mcp.Description("ECharts JSON configuration object"),
			mcp.Required(),
		),
		mcp.WithString("title",
			mcp.Description("Title of the chart page"),
			mcp.Required(),
		),
		mcp.WithNumber("width",
			mcp.Description("Width of the chart in pixels"),
		),
		mcp.WithNumber("height",
			mcp.Description("Height of the chart in pixels"),
		),
	)
	s.AddTool(generateEchartsPage, GenerateEchartsPage)

	// Ensure the static directory exists
	staticDir := getEnv("STATIC_DIR", "static")
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		if err := os.MkdirAll(staticDir, 0755); err != nil {
			log.Fatalf("Failed to create the static directory: %v", err)
		}
	}

	// Read the port configuration
	port := getEnv("PORT", "8989")

	// Create a unified HTTP router
	mux := http.NewServeMux()

	// Serve static files
	mux.Handle("/", http.FileServer(http.Dir(staticDir)))

	// Create the MCP HTTP server
	mcpHandler := server.NewStreamableHTTPServer(s,
		server.WithEndpointPath("/mcp"),
		server.WithSessionIdManager(&server.InsecureStatefulSessionIdManager{}),
		server.WithHeartbeatInterval(5*time.Second),
		server.WithLogger(log.StandardLogger()),
	)

	// Register the MCP handler with the router
	mux.Handle("/mcp", mcpHandler)

	// Build the shared HTTP server
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Start the HTTP server
	go func() {
		log.Infof("Server starting on port %s (MCP endpoint: /mcp, static files: /)", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed to start: %v\n", err)
		}
	}()

	// Configure graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down the server...")

	// Create a shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Gracefully shut down the server
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Info("Server shut down successfully")
}

func GenerateEchartsPage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Safely extract parameters
	args := request.GetArguments()

	inputSchema, ok := args["inputSchema"].(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Parameter 'inputSchema' must be a JSON object"), nil
	}

	title, ok := args["title"].(string)
	if !ok {
		return mcp.NewToolResultError("Parameter 'title' must be a string"), nil
	}

	// GetInt handles type conversions and supplies defaults when the key is missing
	width := request.GetInt("width", 1000)
	height := request.GetInt("height", 600)

	// Encode inputSchema as a JSON string
	optionBytes, err := json.Marshal(inputSchema)
	if err != nil {
		return mcp.NewToolResultError("Failed to serialize inputSchema: " + err.Error()), nil
	}

	// Prepare formatted JSON for display and injection
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, optionBytes, "", "  "); err != nil {
		log.Warnf("Failed to format JSON: %v", err)
		prettyJSON.WriteString(string(optionBytes)) // Fallback to raw string
	}

	data := PageData{
		Title:     title,
		Width:     width,
		Height:    height,
		Option:    template.JS(prettyJSON.String()),
		OptionStr: prettyJSON.String(),
	}

	// Parse and execute the template
	tmpl, err := template.New("echarts").Parse(getTemplate())
	if err != nil {
		return mcp.NewToolResultError("Template parsing failed: " + err.Error()), nil
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return mcp.NewToolResultError("Template execution failed: " + err.Error()), nil
	}

	// Save the HTML file into static/charts
	staticDir := getEnv("STATIC_DIR", "static")
	chartsDir := filepath.Join(staticDir, "charts")
	if err := os.MkdirAll(chartsDir, 0755); err != nil {
		return mcp.NewToolResultError("Failed to create charts directory: " + err.Error()), nil
	}

	fileName := fmt.Sprintf("echarts_%d.html", time.Now().UnixNano())
	filePath := filepath.Join(chartsDir, fileName)

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return mcp.NewToolResultError("Failed to save HTML file: " + err.Error()), nil
	}

	// Build the result URL
	port := getEnv("PORT", "8989")
	publicURL := getEnv("PUBLIC_URL", fmt.Sprintf("http://localhost:%s", port))
	// Ensure publicURL has no trailing slash
	publicURL = strings.TrimSuffix(publicURL, "/")

	resultURL := fmt.Sprintf("%s/charts/%s", publicURL, fileName)
	return mcp.NewToolResultText(resultURL), nil
}

func getTemplate() string {
	// Prefer reading from the file system to support live reload during development
	data, err := os.ReadFile("template.html")
	if err == nil {
		return string(data)
	}
	// Fallback to the embedded template if reading the file fails (for example, in a deployment environment)
	return htmlTemplate
}

// getEnv reads an environment variable or returns the fallback
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// getEnvAsInt reads an environment variable as an integer or returns the fallback when parsing fails
func getEnvAsInt(name string, fallback int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}
