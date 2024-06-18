package api

import (
	"github.com/gin-gonic/gin"
)

type asset struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}

type addAssetsToUniverseRequest struct {
	UniverseName string  `json:"universeName"`
	Assets       []asset `json:"assets"`
}

func (m ApiHandler) addAssetsToUniverse(c *gin.Context) {
	tx, err := m.Db.Begin()
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	defer tx.Rollback()

	out := map[string]string{
		"message": "ok",
	}

	c.JSON(200, out)
}
