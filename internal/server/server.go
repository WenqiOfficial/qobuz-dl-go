package server

import (
	"fmt"
	"net/http"
	"qobuz-dl-go/internal/engine"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

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

		// Set headers for streaming audio
		c.Response().Header().Set(echo.HeaderContentType, "audio/flac")
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"%s.flac\"", trackID))
		
		// Flush headers to client immediately
		c.Response().WriteHeader(http.StatusOK)

		// Create a writer wrapper
		// Note: Echo's Response() implements io.Writer
		err := eng.StreamTrack(c.Request().Context(), trackID, quality, c.Response().Writer, nil)
		if err != nil {
			// Since we already sent 200 OK, we can't send error status code.
			// We can only log error. 
			// In a real app we might want to check metadata first before sending 200.
			fmt.Printf("Stream error: %v\n", err)
			return nil
		}
		
		return nil
	})

	e.Logger.Fatal(e.Start(":" + port))
}
