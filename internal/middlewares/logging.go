package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func LoggerMiddleware(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := uuid.New().String()

		// Add request ID to context and logger
		c.Set("requestID", reqID)
		eventLogger := logger.With().Str("request_id", reqID).Logger()
		c.Set("logger", eventLogger)

		eventLogger.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Msg("Incoming request")

		c.Next()

		eventLogger.Info().
			Int("status", c.Writer.Status()).
			Msg("Request completed")
	}
}
