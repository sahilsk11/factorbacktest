package api

import (
	"alpha/internal"
	"alpha/internal/app"
	"alpha/internal/repository"
	"database/sql"
	"fmt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type ApiHandler struct {
	Db                     *sql.DB
	BacktestHandler        app.BacktestHandler
	BenchmarkHandler       internal.BenchmarkHandler
	UserStrategyRepository repository.UserStrategyRepository
	ContactRepository      repository.ContactRepository
}

func (m ApiHandler) StartApi(port int) error {
	router := gin.Default()
	router.Use(cors.Default())

	router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(200, map[string]string{"message": "welcome to alpha"})
	})
	router.POST("/backtest", m.backtest)
	router.POST("/benchmark", m.benchmark)
	router.POST("/contact", m.contact)

	return router.Run(fmt.Sprintf(":%d", port))
}

func returnErrorJson(err error, c *gin.Context) {
	fmt.Println(err.Error())
	c.AbortWithStatusJSON(500, gin.H{
		"error": err.Error(),
	})
}

func returnErrorJsonCode(err error, c *gin.Context, code int) {
	fmt.Println(err.Error())
	c.AbortWithStatusJSON(code, gin.H{
		"error": err.Error(),
	})
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	}
}
