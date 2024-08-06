package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func (m ApiHandler) getPublishedStrategies(c *gin.Context) {
	returnErrorJson(fmt.Errorf("not implemented"), c)
}
