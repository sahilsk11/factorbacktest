package main

import (
	"database/sql"
	"net/http"
	"os"
	"sort"

	"factorbacktest/internal/testseed"

	"github.com/gin-gonic/gin"
)

// MountAdmin registers the test-only admin routes under /__test__/ on the
// given engine. It is only safe to call when ALPHA_ENV=test; it panics
// otherwise so that any accidental use in a non-test context fails loudly.
//
// This file lives in package main of cmd/test-api so it is never linked
// into the production cmd/api binary.
func MountAdmin(engine *gin.Engine, db *sql.DB, reg *testseed.Registry) {
	if os.Getenv("ALPHA_ENV") != "test" {
		panic("cmd/test-api: MountAdmin called with ALPHA_ENV != \"test\"")
	}

	group := engine.Group("/__test__")

	group.GET("/fixtures", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"fixtures": reg.List()})
	})

	group.POST("/reset", func(c *gin.Context) {
		if err := testseed.Reset(c.Request.Context(), db); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	})

	group.POST("/fixtures", func(c *gin.Context) {
		var req struct {
			Fixtures []string `json:"fixtures"`
			Reset    bool     `json:"reset"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()

		if req.Reset {
			if err := testseed.Reset(ctx, db); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		ids, err := reg.Apply(ctx, db, req.Fixtures)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		applied := make([]string, 0, len(ids))
		for name := range ids {
			applied = append(applied, name)
		}
		sort.Strings(applied)

		c.JSON(http.StatusOK, gin.H{
			"applied": applied,
			"ids":     ids,
		})
	})
}
