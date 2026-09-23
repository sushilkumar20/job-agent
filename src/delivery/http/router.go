package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Config struct {
	Addr          string
	AllowedOrigin string
}

// NewServer builds the HTTP server. Routes live here; the handlers hold no
// routing knowledge and the usecase layer holds no HTTP knowledge.
func NewServer(cfg Config, h *Handler) *http.Server {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(recoverPanic(), requestLogger(), cors(cfg.AllowedOrigin))

	r.GET("/health", h.health)

	api := r.Group("/api")
	{
		api.POST("/analyze", h.analyze)
	}

	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		// An analysis is several model calls; the default would cut it off.
		WriteTimeout: 15 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}
}
