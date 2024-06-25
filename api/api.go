package api

import (
	"alpha/internal"
	"alpha/internal/app"
	"alpha/internal/db/models/postgres/public/model"
	"alpha/internal/repository"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ApiHandler struct {
	Db                     *sql.DB
	BacktestHandler        app.BacktestHandler
	BenchmarkHandler       internal.BenchmarkHandler
	UserStrategyRepository repository.UserStrategyRepository
	ContactRepository      repository.ContactRepository
	GptRepository          repository.GptRepository
	ApiRequestRepository   repository.ApiRequestRepository
}

func int64Ptr(i int64) *int64 {
	return &i
}
func int32Ptr(i int32) *int32 {
	return &i
}
func strPtr(s string) *string {
	return &s
}

func (m ApiHandler) StartApi(port int) error {
	router := gin.Default()
	router.Use(cors.Default())
	router.Use(m.logRequestMiddlware)

	router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(200, map[string]string{"message": "welcome to alpha"})
	})
	router.POST("/backtest", m.backtest)
	router.POST("/benchmark", m.benchmark)
	router.POST("/contact", m.contact)
	router.POST("/constructFactorEquation", m.constructFactorEquation)

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

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (m ApiHandler) logRequestMiddlware(ctx *gin.Context) {
	w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: ctx.Writer}
	ctx.Writer = w

	body, err := ctx.GetRawData()
	if err != nil {
		log.Println(fmt.Errorf("failed to get raw data: %w", err))
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(body))

	type userIdBody struct {
		UserID uuid.UUID `json:"userID"`
	}

	reqBody := userIdBody{}
	err = json.Unmarshal(body, &reqBody)
	if err != nil {
		log.Println(fmt.Errorf("failed to get req body: %w", err))
	}
	var userID *uuid.UUID
	if reqBody.UserID != uuid.Nil {
		userID = &reqBody.UserID
	}

	start := time.Now().UTC()
	req, err := m.ApiRequestRepository.Add(m.Db, model.APIRequest{
		UserID:      userID,
		IPAddress:   strPtr(ctx.ClientIP()),
		Method:      ctx.Request.Method,
		Route:       ctx.Request.URL.Path,
		RequestBody: strPtr(string(body)),
		StartTs:     start,
	})
	if err != nil {
		log.Println(err)
	}

	ctx.Next()

	if req != nil {
		req.DurationMs = int64Ptr(time.Since(start).Milliseconds())
		req.StatusCode = int32Ptr(int32(ctx.Writer.Status()))
		req.ResponseBody = strPtr(w.body.String())

		err = m.ApiRequestRepository.Update(m.Db, *req)
		if err != nil {
			log.Println(err)
		}
	}
}
