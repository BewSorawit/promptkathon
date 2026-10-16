package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"flood-management-system/backend/internal/data"
	"flood-management-system/backend/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func Router(db *store.Store, graphURL string) *gin.Engine {
	r := gin.Default()
	graphClient := &http.Client{Timeout: 12 * time.Second}
	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.DB.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	v1 := r.Group("/api/v1")
	v1.GET("/stations", func(c *gin.Context) {
		stations, err := db.Stations(c.Request.Context(), c.Query("province"))
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"stations": stations, "source": "ThaiWater", "generated_at": time.Now().UTC()})
	})
	v1.GET("/stations/:id", func(c *gin.Context) {
		id, ok := stationID(c)
		if !ok {
			return
		}
		station, err := db.Station(c.Request.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "station not found"})
			return
		}
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, station)
	})
	v1.GET("/stations/:id/observations", func(c *gin.Context) {
		id, ok := stationID(c)
		if !ok {
			return
		}
		hours := 48
		if h := c.Query("hours"); h != "" {
			parsed, err := strconv.Atoi(h)
			if err != nil || parsed < 1 || parsed > 168 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "hours must be 1..168"})
				return
			}
			hours = parsed
		}
		_, stationErr := db.Station(c.Request.Context(), id)
		if errors.Is(stationErr, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "station not found"})
			return
		}
		if stationErr != nil {
			fail(c, stationErr)
			return
		}
		claimed, claimErr := db.ClaimHistoryFetch(c.Request.Context(), id)
		if claimErr != nil {
			fail(c, claimErr)
			return
		}
		if claimed {
			points, graphErr := data.FetchGraph(c.Request.Context(), graphClient, graphURL, id)
			if graphErr == nil {
				_, graphErr = db.ImportHistory(c.Request.Context(), points, "ThaiWater graph")
			}
			if graphErr != nil {
				_ = c.Error(graphErr)
			}
		}
		rows, err := db.Observations(c.Request.Context(), id, time.Now().Add(-time.Duration(hours)*time.Hour))
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"observations": rows})
	})
	v1.GET("/stations/:id/forecast", func(c *gin.Context) {
		id, ok := stationID(c)
		if !ok {
			return
		}
		station, err := db.Station(c.Request.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "station not found"})
			return
		}
		if err != nil {
			fail(c, err)
			return
		}
		if station.IsStale {
			c.JSON(http.StatusOK, gin.H{"status": "stale", "forecast": nil})
			return
		}
		forecast, err := db.Forecast(c.Request.Context(), id)
		if err != nil {
			fail(c, err)
			return
		}
		if forecast == nil || station.ObservedAt == nil || forecast.OriginAt.Before(station.ObservedAt.Add(-10*time.Minute)) || time.Since(forecast.OriginAt) > 30*time.Minute {
			c.JSON(http.StatusOK, gin.H{"status": "unavailable", "forecast": nil})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "available", "forecast": forecast})
	})
	v1.GET("/source-status", func(c *gin.Context) {
		sync, err := db.SourceSync(c.Request.Context())
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"sync": sync})
	})
	return r
}
func stationID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid station id"})
		return 0, false
	}
	return id, true
}
func fail(c *gin.Context, err error) {
	_ = c.Error(err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}
