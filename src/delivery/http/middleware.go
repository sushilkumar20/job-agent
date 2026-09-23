package http

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// requestLogger records method, path, status and duration. Analysis requests
// take minutes, so the duration is the useful part.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("%s %s %d %s", c.Request.Method, c.Request.URL.Path,
			c.Writer.Status(), time.Since(start).Round(time.Millisecond))
	}
}

// recoverPanic keeps one bad request from taking the process down, and returns
// a generic message rather than leaking a stack trace to the caller.
func recoverPanic() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic handling %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
				c.AbortWithStatusJSON(http.StatusInternalServerError,
					gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}

// cors allows the browser UI to call this API from a different port.
func cors(allowedOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
