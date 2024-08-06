package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func (m ApiHandler) isStrategyBookmarked(c *gin.Context) {
	returnErrorJson(fmt.Errorf("not implemented"), c)
}

func (m ApiHandler) bookmarkStrategy(c *gin.Context) {
	returnErrorJson(fmt.Errorf("not implemented"), c)
}
