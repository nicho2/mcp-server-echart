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

//go:embed docs/swagger.json
var swaggerSpec []byte

// PageData holds data for the HTML template
type PageData struct {
	Title     string
	Width     int
	Height    int
	Option    template.JS
	OptionStr string
}

// GenerateEchartsPageParams represents the validated parameters required
// to render and persist an ECharts page.
type GenerateEchartsPageParams struct {
	Title       string
	InputSchema map[string]interface{}
	Width       int
	Height      int
}

// GenerateEchartsPageRequest models the REST API request body for
// generating an ECharts page.
type GenerateEchartsPageRequest struct {
	Title       string                 `json:"title"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Width       *int                   `json:"width,omitempty"`
	Height      *int                   `json:"height,omitempty"`
}

// GenerateEchartsPageResponse represents the REST API response body on success.
type GenerateEchartsPageResponse struct {
	URL string `json:"url"`
}

const (
	defaultChartWidth  = 1000
	defaultChartHeight = 600
)

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

	// Configure the REST API server
	apiPort := getEnv("API_PORT", "8990")
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/GenerateEchartsPage", handleGenerateEchartsPageAPI)
	apiMux.HandleFunc("/swagger.json", swaggerSpecHandler)
	apiMux.HandleFunc("/docs", swaggerUIHandler("/swagger.json"))
	apiServer := &http.Server{
		Addr:    ":" + apiPort,
		Handler: apiMux,
	}

	// Start the HTTP server
	go func() {
		log.Infof("Server starting on port %s (MCP endpoint: /mcp, static files: /)", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed to start: %v\n", err)
		}
	}()

	// Start the REST API server
	go func() {
		log.Infof("REST API server starting on port %s (Swagger UI: /docs)", apiPort)
		if err := apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("REST API server failed to start: %v\n", err)
		}
	}()

	// Configure graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down the servers...")

	// Create a shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Gracefully shut down the servers
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	if err := apiServer.Shutdown(ctx); err != nil {
		log.Fatalf("REST API server shutdown error: %v", err)
	}

	log.Info("Servers shut down successfully")
}

func GenerateEchartsPage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Safely extract parameters
	args := request.GetArguments()

	rawInputSchema, exists := args["inputSchema"]
	if !exists {
		return mcp.NewToolResultError("Parameter 'inputSchema' must be provided"), nil
	}

	var inputSchema map[string]interface{}
	switch value := rawInputSchema.(type) {
	case map[string]interface{}:
		inputSchema = value
	case string:
		if err := json.Unmarshal([]byte(value), &inputSchema); err != nil {
			return mcp.NewToolResultError("Parameter 'inputSchema' must be a JSON object or JSON string"), nil
		}
	default:
		return mcp.NewToolResultError("Parameter 'inputSchema' must be a JSON object or JSON string"), nil
	}

	title, ok := args["title"].(string)
	if !ok {
		return mcp.NewToolResultError("Parameter 'title' must be a string"), nil
	}

	// GetInt handles type conversions and supplies defaults when the key is missing
	width := request.GetInt("width", defaultChartWidth)
	height := request.GetInt("height", defaultChartHeight)

	params := GenerateEchartsPageParams{
		Title:       title,
		InputSchema: inputSchema,
		Width:       width,
		Height:      height,
	}

	resultURL, err := renderAndPersistEchartsPage(params)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(resultURL), nil
}

func renderAndPersistEchartsPage(params GenerateEchartsPageParams) (string, error) {
	if params.InputSchema == nil {
		return "", fmt.Errorf("Parameter 'inputSchema' must be a JSON object")
	}

	if params.Title == "" {
		return "", fmt.Errorf("Parameter 'title' must be a string")
	}

	width := params.Width
	if width == 0 {
		width = defaultChartWidth
	}

	height := params.Height
	if height == 0 {
		height = defaultChartHeight
	}

	// Encode inputSchema as a JSON string
	optionBytes, err := json.Marshal(params.InputSchema)
	if err != nil {
		return "", fmt.Errorf("Failed to serialize inputSchema: %w", err)
	}

	// Prepare formatted JSON for display and injection
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, optionBytes, "", "  "); err != nil {
		log.Warnf("Failed to format JSON: %v", err)
		prettyJSON.Write(optionBytes)
	}

	data := PageData{
		Title:     params.Title,
		Width:     width,
		Height:    height,
		Option:    template.JS(prettyJSON.String()),
		OptionStr: prettyJSON.String(),
	}

	// Parse and execute the template
	tmpl, err := template.New("echarts").Parse(getTemplate())
	if err != nil {
		return "", fmt.Errorf("Template parsing failed: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("Template execution failed: %w", err)
	}

	// Save the HTML file into static/charts
	staticDir := getEnv("STATIC_DIR", "static")
	chartsDir := filepath.Join(staticDir, "charts")
	if err := os.MkdirAll(chartsDir, 0755); err != nil {
		return "", fmt.Errorf("Failed to create charts directory: %w", err)
	}

	fileName := fmt.Sprintf("echarts_%d.html", time.Now().UnixNano())
	filePath := filepath.Join(chartsDir, fileName)

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("Failed to save HTML file: %w", err)
	}

	// Build the result URL
	port := getEnv("PORT", "8989")
	publicURL := getEnv("PUBLIC_URL", fmt.Sprintf("http://localhost:%s", port))
	publicURL = strings.TrimSuffix(publicURL, "/")

	resultURL := fmt.Sprintf("%s/charts/%s", publicURL, fileName)
	return resultURL, nil
}

func handleGenerateEchartsPageAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Warnf("Failed to close request body: %v", err)
		}
	}()

	var req GenerateEchartsPageRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if req.Title == "" {
		respondWithError(w, http.StatusBadRequest, "Parameter 'title' must be a non-empty string")
		return
	}

	if req.InputSchema == nil {
		respondWithError(w, http.StatusBadRequest, "Parameter 'inputSchema' must be an object")
		return
	}

	width := defaultChartWidth
	if req.Width != nil {
		width = *req.Width
	}

	height := defaultChartHeight
	if req.Height != nil {
		height = *req.Height
	}

	params := GenerateEchartsPageParams{
		Title:       req.Title,
		InputSchema: req.InputSchema,
		Width:       width,
		Height:      height,
	}

	resultURL, err := renderAndPersistEchartsPage(params)
	if err != nil {
		log.Errorf("Failed to generate ECharts page via REST API: %v", err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, GenerateEchartsPageResponse{URL: resultURL})
}

func swaggerSpecHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(swaggerSpec); err != nil {
		log.Errorf("Failed to write swagger specification: %v", err)
	}
}

func swaggerUIHandler(swaggerEndpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>mcp-server-echart API Documentation</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin="anonymous"></script>
    <script>
      window.onload = () => {
        window.ui = SwaggerUIBundle({
          url: '%s',
          dom_id: '#swagger-ui',
          presets: [SwaggerUIBundle.presets.apis],
        });
      };
    </script>
  </body>
</html>`, swaggerEndpoint)

		if _, err := fmt.Fprint(w, html); err != nil {
			log.Errorf("Failed to write swagger UI response: %v", err)
		}
	}
}

func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Errorf("Failed to encode JSON response: %v", err)
	}
}

func respondWithError(w http.ResponseWriter, status int, message string) {
	respondWithJSON(w, status, map[string]string{"error": message})
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
