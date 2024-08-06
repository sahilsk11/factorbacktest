package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"factorbacktest/internal"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"
	l1_service "factorbacktest/internal/service/l1"
	l3_service "factorbacktest/internal/service/l3"
	googleauth "factorbacktest/pkg/google-auth"
	"fmt"
	"io"
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
	BacktestHandler              l3_service.BacktestHandler
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
	StrategyRepository           repository.StrategyRepository
	InvestmentRepository         repository.InvestmentRepository
	TradingService               l1_service.TradeService
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

func (m ApiHandler) InitializeRouterEngine(ctx context.Context) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())
	// engine.Use(gin.Logger())
	lg := logger.FromContext(ctx)

	engine.Use(func(c *gin.Context) {
		l := lg.With(
			"method", c.Request.Method,
			"route", c.Request.URL.Path,
		)
		c.Set(logger.ContextKey, l)
	})
	engine.Use(blockBots)
	engine.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:3000",
			"https://factorbacktest.net",
		},
		AllowHeaders: []string{"Authorization", "Content-Type"},
	}))
	engine.Use(m.getGoogleAuthMiddleware)
	engine.Use(m.logRequestMiddlware)
	engine.Use(func(ctx *gin.Context) {
		logger.FromContext(ctx).Info("new request")
	})

	engine.GET("/", func(ctx *gin.Context) {
		ctx.JSON(200, map[string]string{"message": "welcome to alpha"})
	})

	engine.POST("/backtest", m.backtest)
	engine.POST("/benchmark", m.benchmark)
	engine.POST("/contact", m.contact)
	engine.POST("/constructFactorEquation", m.constructFactorEquation)
	engine.GET("/usageStats", func(ctx *gin.Context) {
		result, err := repository.GetUsageStats(m.Db)
		if err != nil {
			returnErrorJson(err, ctx)
			return
		}
		ctx.JSON(200, result)
	})
	engine.GET("/assetUniverses", m.getAssetUniverses)

	engine.POST("/backtestBondPortfolio", m.backtestBondPortfolio)
	engine.POST("/updatePrices", m.updatePrices)
	engine.POST("/addAssetsToUniverse", m.addAssetsToUniverse)
	engine.POST("/bookmarkStrategy", m.bookmarkStrategy)
	engine.POST("/isStrategyBookmarked", m.isStrategyBookmarked)
	engine.GET("/savedStrategies", m.getSavedStrategies)
	engine.POST("/investInStrategy", m.investInStrategy)
	engine.GET("/activeInvestments", m.getInvestments)
	engine.GET("/publishedStrategies", m.getPublishedStrategies)

	return engine
}

func (m ApiHandler) StartApi(ctx context.Context, port int) error {
	engine := m.InitializeRouterEngine(ctx)
	return engine.Run(fmt.Sprintf(":%d", port))
}

func returnErrorJson(err error, c *gin.Context) {
	returnErrorJsonCode(err, c, 500)
}

func returnErrorJsonCode(err error, c *gin.Context, code int) {
	lg := logger.FromContext(c)
	lg.Errorf("[%d] %s", code, err.Error())
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
	lg := logger.FromContext(ctx)
	w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: ctx.Writer}
	ctx.Writer = w

	method := ctx.Request.Method

	var requestBody *string
	var userID *uuid.UUID

	if method == "POST" {
		body, err := ctx.GetRawData()
		if err != nil {
			lg.Warnf("failed to get raw data: %s", err.Error())
		}
		ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
		requestBody = strPtr(string(body))

		type userIdBody struct {
			UserID uuid.UUID `json:"userID"`
		}

		reqBody := userIdBody{}
		err = json.Unmarshal(body, &reqBody)
		if err != nil {
			lg.Warnf("failed to get req body: %s", err.Error())
		}

		if reqBody.UserID != uuid.Nil {
			userID = &reqBody.UserID
		}
	}
	if method == "GET" {
		userID = GetUserIDUrlParam(ctx)
	}

	if userID != nil {
		lg = lg.With("userID", userID.String())
		ctx.Set(logger.ContextKey, lg)
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
		lg.Warn(err.Error())
	}

	lg = lg.With("requestID", req.RequestID.String())
	ctx.Set(logger.ContextKey, lg)

	ctx.Next()

	if req != nil {
		req.DurationMs = int64Ptr(time.Since(start).Milliseconds())
		req.StatusCode = int32Ptr(int32(ctx.Writer.Status()))
		req.ResponseBody = strPtr(w.body.String())

		err = m.ApiRequestRepository.Update(m.Db, *req)
		if err != nil {
			lg.Error(err)
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
		returnErrorJsonCode(fmt.Errorf("misformatted auth"), c, 403)
		return
	}
	jwt = jwt[len("Bearer "):]
	userDetails, err := googleauth.GetUserDetails(jwt)
	if err != nil {
		returnErrorJsonCode(fmt.Errorf("failed google auth: %s", err.Error()), c, 403)
		return
	}

	user, err := m.UserAccountRepository.GetOrCreate(*userDetails)
	if err != nil {
		returnErrorJsonCode(fmt.Errorf("failed create user: %s", err.Error()), c, 500)
		return
	}

	c.Set("userAccountID", user.UserAccountID.String())

	lg := logger.FromContext(c).With(
		"userAccountID", user.UserAccountID.String(),
	)
	c.Set(logger.ContextKey, lg)

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
