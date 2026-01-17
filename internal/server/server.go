// Package server provides the web server for streaming and API access.
// It uses Echo framework for HTTP handling.
package server

import (
	"fmt"
	"net/http"
	"qobuz-dl-go/internal/engine"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Start initializes and starts the web server on the specified port.
// It provides endpoints for health checks and audio streaming.
func Start(eng *engine.Engine, port string) {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Qobuz-DL Go Engine Running")
	})

	e.GET("/stream/:trackID", func(c echo.Context) error {
		trackID := c.Param("trackID")
		qualityStr := c.QueryParam("quality")
		quality := 6
		if qualityStr != "" {
			if q, err := strconv.Atoi(qualityStr); err == nil {
				quality = q
			}
		}

		// Stream track - headers will be set based on actual response
		streamInfo, err := eng.StreamTrack(c.Request().Context(), trackID, quality, c.Response().Writer, nil)
		if err != nil {
			// If streaming failed before any data was sent, return error
			if streamInfo == nil {
				return c.String(http.StatusInternalServerError, fmt.Sprintf("Stream error: %v", err))
			}
			// Otherwise log it (data may have been partially sent)
			fmt.Printf("Stream error: %v\n", err)
			return nil
		}

		return nil
	})

	e.Logger.Fatal(e.Start(":" + port))
}
