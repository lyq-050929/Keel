package api

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/smartcs/go-impl/agent"
	"github.com/smartcs/go-impl/config"
	"github.com/smartcs/go-impl/mcp"
	"github.com/smartcs/go-impl/memory"
	"github.com/smartcs/go-impl/tracing"

	"github.com/gin-gonic/gin"
)

// Server HTTP API服务（基于Gin框架）
type Server struct {
	supervisor   *agent.SupervisorAgent
	shortTermMem *memory.ShortTermMemory
	longTermMem  *memory.LongTermMemory
	toolServer   *mcp.MCPToolServer
	cfg          config.Config
	startedAt    time.Time
	requestCount atomic.Int64
	engine       *gin.Engine
}

type ChatRequest struct {
	Message   string `json:"message" binding:"required"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

func NewServer(
	supervisor *agent.SupervisorAgent,
	stm *memory.ShortTermMemory,
	ltm *memory.LongTermMemory,
	toolServer *mcp.MCPToolServer,
	cfg config.Config,
) *Server {
	s := &Server{
		supervisor:   supervisor,
		shortTermMem: stm,
		longTermMem:  ltm,
		toolServer:   toolServer,
		cfg:          cfg,
		startedAt:    time.Now(),
	}

	gin.SetMode(gin.ReleaseMode)
	s.engine = gin.Default()
	s.engine.Use(s.requestContextMiddleware())
	s.engine.Use(s.metricsMiddleware())
	if cfg.APIKey != "" {
		s.engine.Use(s.apiKeyMiddleware())
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	api := s.engine.Group("/api")
	{
		api.POST("/chat", s.handleChat)
		api.GET("/history/:sessionId", s.handleHistory)
		api.GET("/tools", s.handleListTools)
		api.GET("/metrics", s.handleMetrics)
	}
	s.engine.GET("/health", s.handleHealth)
	s.engine.GET("/ready", s.handleReady)
}

func (s *Server) handleChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == "" {
		req.UserID = "anonymous"
	}
	if req.SessionID == "" {
		req.SessionID = generateSessionID()
	}

	s.shortTermMem.AddMessage(req.SessionID, "user", req.Message)

	state := agent.NewState(req.UserID, req.SessionID, req.Message)
	result := s.supervisor.Orchestrate(state)

	s.shortTermMem.AddMessage(req.SessionID, "assistant", result.FinalResponse)

	c.JSON(http.StatusOK, gin.H{
		"response":          result.FinalResponse,
		"session_id":        req.SessionID,
		"intent":            result.Intent,
		"compliance_passed": result.CompliancePassed,
	})
}

func (s *Server) handleHistory(c *gin.Context) {
	sessionID := c.Param("sessionId")
	history := s.shortTermMem.GetHistory(sessionID)
	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"messages":   history,
	})
}

func (s *Server) handleListTools(c *gin.Context) {
	tools := []mcp.ToolDefinition{}
	if s.toolServer != nil {
		tools = s.toolServer.ListTools()
	}
	c.JSON(http.StatusOK, gin.H{"tools": tools})
}

func (s *Server) handleMetrics(c *gin.Context) {
	uptime := time.Since(s.startedAt).Seconds()
	c.JSON(http.StatusOK, gin.H{
		"agent_metrics": tracing.GetMetrics(),
		"request_count": s.requestCount.Load(),
		"uptime_sec":    uptime,
	})
}

func (s *Server) handleHealth(c *gin.Context) {
	toolCount := 0
	if s.toolServer != nil {
		toolCount = len(s.toolServer.ListTools())
	}
	c.JSON(http.StatusOK, gin.H{
		"status":     "healthy",
		"version":    "1.0.0",
		"tool_count": toolCount,
	})
}

func (s *Server) handleReady(c *gin.Context) {
	toolCount := 0
	if s.toolServer != nil {
		toolCount = len(s.toolServer.ListTools())
	}
	c.JSON(http.StatusOK, gin.H{
		"status":      "ready",
		"data_dir":    s.cfg.DataDir,
		"has_api_key": s.cfg.APIKey != "",
		"tool_count":  toolCount,
		"started_at":  s.startedAt.Format(time.RFC3339),
	})
}

func (s *Server) Run(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.engine,
		ReadHeaderTimeout: s.cfg.RequestTimeout,
		ReadTimeout:       s.cfg.RequestTimeout,
		WriteTimeout:      s.cfg.RequestTimeout,
		IdleTimeout:       2 * s.cfg.RequestTimeout,
	}
	return srv.ListenAndServe()
}

func generateSessionID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "sess-fallback"
	}
	return "sess-" + hex.EncodeToString(buf)
}

func (s *Server) requestContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateSessionID()
		}
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

func (s *Server) metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		s.requestCount.Add(1)
		c.Next()
		log.Printf("request_id=%s method=%s path=%s status=%d latency_ms=%d",
			getRequestID(c), c.Request.Method, c.FullPath(), c.Writer.Status(), time.Since(start).Milliseconds())
	}
}

func (s *Server) apiKeyMiddleware() gin.HandlerFunc {
	expected := s.cfg.APIKey
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/ready" {
			c.Next()
			return
		}

		provided := c.GetHeader("X-API-Key")
		if provided == "" {
			auth := c.GetHeader("Authorization")
			if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				provided = strings.TrimSpace(auth[7:])
			}
		}

		if provided != expected {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func getRequestID(c *gin.Context) string {
	if v, ok := c.Get("request_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
