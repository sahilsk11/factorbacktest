package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"factorbacktest/internal"
	"factorbacktest/internal/app"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"
	l1_service "factorbacktest/internal/service/l1"
	l3_service "factorbacktest/internal/service/l3"
	googleauth "factorbacktest/pkg/google-auth"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ApiHandler struct {
	Db                           *sql.DB
	BacktestHandler              app.BacktestHandler
	BenchmarkHandler             internal.BenchmarkHandler
	UserStrategyRepository       repository.UserStrategyRepository
	ContactRepository            repository.ContactRepository
	GptRepository                repository.GptRepository
	ApiRequestRepository         repository.ApiRequestRepository
	LatencencyTrackingRepository repository.LatencyTrackingRepository
	PriceService                 l1_service.PriceService
	InvestmentService            l3_service.InvestmentService
	TickerRepository             repository.TickerRepository
	PriceRepository              repository.AdjustedPriceRepository
	AssetUniverseRepository      repository.AssetUniverseRepository
	UserAccountRepository        repository.UserAccountRepository
	SavedStrategyRepository      repository.SavedStrategyRepository
	StrategyInvestmentRepository repository.StrategyInvestmentRepository
	RebalancerHandler            app.RebalancerHandler
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

func (m ApiHandler) InitializeRouterEngine() *gin.Engine {
	router := gin.Default()

	router.Use(blockBots)
	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:3000",
			"https://factorbacktest.net",
		},
		AllowHeaders: []string{"Authorization", "Content-Type"},
	}))
	router.Use(m.getGoogleAuthMiddleware)
	router.Use(m.logRequestMiddlware)

	router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(200, map[string]string{"message": "welcome to alpha"})
	})

	router.POST("/backtest", m.backtest)
	router.POST("/benchmark", m.benchmark)
	router.POST("/contact", m.contact)
	router.POST("/constructFactorEquation", m.constructFactorEquation)
	router.GET("/usageStats", func(ctx *gin.Context) {
		result, err := repository.GetUsageStats(m.Db)
		if err != nil {
			returnErrorJson(err, ctx)
			return
		}
		ctx.JSON(200, result)
	})
	router.GET("/assetUniverses", m.getAssetUniverses)

	router.POST("/backtestBondPortfolio", m.backtestBondPortfolio)
	router.POST("/updatePrices", m.updatePrices)
	router.POST("/addAssetsToUniverse", m.addAssetsToUniverse)
	router.POST("/bookmarkStrategy", m.bookmarkStrategy)
	router.POST("/isStrategyBookmarked", m.isStrategyBookmarked)
	router.GET("/savedStrategies", m.getSavedStrategies)
	router.POST("/investInStrategy", m.investInStrategy)
	router.GET("/activeInvestments", m.getInvestments)

	return router
}

func (m ApiHandler) StartApi(port int) error {
	router := m.InitializeRouterEngine()
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

func blockBots(c *gin.Context) {
	clientIP := c.ClientIP()
	blockedIps := []string{"172.31.45.22"}
	for _, ip := range blockedIps {
		if ip == clientIP {
			c.JSON(http.StatusForbidden, gin.H{"message": "Access denied"})
			c.Abort()
			return
		}
	}
	c.Next()
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

	method := ctx.Request.Method

	var requestBody *string
	var userID *uuid.UUID

	if method == "POST" {
		body, err := ctx.GetRawData()
		if err != nil {
			log.Println(fmt.Errorf("failed to get raw data: %w", err))
		}
		ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
		requestBody = strPtr(string(body))

		type userIdBody struct {
			UserID uuid.UUID `json:"userID"`
		}

		reqBody := userIdBody{}
		err = json.Unmarshal(body, &reqBody)
		if err != nil {
			log.Println(fmt.Errorf("failed to get req body: %w", err))
		}

		if reqBody.UserID != uuid.Nil {
			userID = &reqBody.UserID
		}
	}
	if method == "GET" {
		userID = GetUserIDUrlParam(ctx)
	}

	var userAccountID *uuid.UUID
	if id, ok := ctx.Get("userAccountID"); ok {
		if idStr, ok := id.(string); ok {
			if uid, err := uuid.Parse(idStr); err == nil {
				userAccountID = &uid
			}
		}
	}

	start := time.Now().UTC()
	commit := os.Getenv("commit_hash")
	req, err := m.ApiRequestRepository.Add(m.Db, model.APIRequest{
		UserID:        userID,
		IPAddress:     strPtr(ctx.ClientIP()),
		Method:        method,
		Route:         ctx.Request.URL.Path,
		RequestBody:   requestBody,
		StartTs:       start,
		Version:       &commit,
		UserAccountID: userAccountID,
	})
	if err != nil {
		log.Println(err)
	}

	ctx.Set("requestID", req.RequestID.String())
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

func (m ApiHandler) getGoogleAuthMiddleware(c *gin.Context) {
	jwt := c.GetHeader("Authorization")
	if jwt == "" {
		c.Next()
		return
	}
	if !strings.HasPrefix(jwt, "Bearer ") {
		c.AbortWithStatusJSON(403, map[string]string{"error": "misformatted auth"})
		return
	}
	jwt = jwt[len("Bearer "):]
	userDetails, err := googleauth.GetUserDetails(jwt)
	if err != nil {
		c.AbortWithStatusJSON(403, map[string]string{"error": fmt.Sprintf("failed google auth: %s", err.Error())})
		return
	}

	user, err := m.UserAccountRepository.GetOrCreate(*userDetails)
	if err != nil {
		c.AbortWithStatusJSON(500, map[string]string{"error": fmt.Sprintf("failed create user: %s", err.Error())})
		return
	}

	c.Set("userAccountID", user.UserAccountID.String())

	c.Next()
}

func GetUserIDUrlParam(ctx *gin.Context) *uuid.UUID {
	urlParams := ctx.Request.URL.Query()

	urlUserID := urlParams.Get("id")
	if urlUserID == "" {
		urlUserID = urlParams.Get("userID")
	}

	id, err := uuid.Parse(urlUserID)
	if err == nil {
		return &id
	}

	return nil
}
