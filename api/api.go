package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"factorbacktest/internal"
	"factorbacktest/internal/app"
	"factorbacktest/internal/auth"
	"factorbacktest/internal/data"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"factorbacktest/internal/util"
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
	Port                         int
	Db                           *sql.DB
	BacktestHandler              service.BacktestHandler
	BenchmarkHandler             internal.BenchmarkHandler
	UserStrategyRepository       repository.UserStrategyRepository
	ContactRepository            repository.ContactRepository
	GptRepository                repository.GptRepository
	ApiRequestRepository         repository.ApiRequestRepository
	LatencencyTrackingRepository repository.LatencyTrackingRepository
	PriceService                 data.PriceService
	InvestmentService            service.InvestmentService
	TickerRepository             repository.TickerRepository
	PriceRepository              repository.AdjustedPriceRepository
	AssetUniverseRepository      repository.AssetUniverseRepository
	UserAccountRepository        repository.UserAccountRepository
	StrategyRepository           repository.StrategyRepository
	InvestmentRepository         repository.InvestmentRepository
	TradingService               service.TradeService
	StrategyService              service.StrategyService
	StrategySummaryApp           app.StrategySummaryApp
	JwtDecodeToken               string
	// BetterAuthJwksURL is the URL the Go API hits to fetch the JWKS used to
	// verify Better Auth-issued JWTs. In production it points at the local
	// auth-service sidecar (default `http://127.0.0.1:3001/api/auth/jwks`).
	BetterAuthJwksURL string
	// BetterAuthExpectedIssuer is the value the auth-service stamps as `iss`
	// on every JWT (its `baseURL`). When set, JWTs whose `iss` doesn't match
	// are rejected. Defense in depth in case the JWKS URL is misconfigured.
	BetterAuthExpectedIssuer string

	// AuthService is the new custom Go auth package. When non-nil, its
	// routes are mounted at /auth/* and its session-cookie middleware
	// runs BEFORE the legacy JWT middleware (so cookie auth wins when
	// both are present). When nil (e.g. during local dev without the
	// secrets configured), /auth/* is unmounted and the API behaves as
	// before. Plumbing it through `nil` is the supported "off" state.
	AuthService *auth.Service

	AlpacaRepository repository.AlpacaRepository
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
	// CORS and the auth package's `requireOrigin` middleware MUST use the
	// same allowlist or one will accept what the other rejects. Built in
	// auth.AppOrigins so both call sites share the source of truth.
	allowedOrigins := auth.AppOrigins()
	engine.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders: []string{"Authorization", "Content-Type", "Cookie"},
		// Better Auth issues HttpOnly session cookies. The browser will only
		// attach them on cross-origin requests when AllowCredentials is true
		// AND the origin is not a wildcard (which we already enforce).
		AllowCredentials: true,
		ExposeHeaders:    []string{"Set-Cookie"},
	}))
	// Reverse-proxy /api/auth/* to the local Better Auth sidecar BEFORE the
	// JWT middleware. The auth-service handles its own request validation
	// (CSRF, OTPs, OAuth callbacks) and we don't want our JWT middleware
	// rejecting unauthenticated calls like sign-in or token refresh.
	authProxy, err := newBetterAuthProxy()
	if err != nil {
		lg.Errorf("failed to build better-auth proxy: %v", err)
	} else {
		engine.Any("/api/auth/*proxyPath", authProxy)
	}

	// New custom Go auth: install the cookie middleware FIRST, then mount
	// /auth/* routes. Gin doesn't apply middleware retroactively to routes
	// registered before Use(); the middleware would otherwise miss the
	// /auth/session handler that reads userAccountID from context.
	// Order matters:
	//   1. CORS (above) so preflight works for /auth/*
	//   2. /api/auth/* proxy (above) so Better Auth keeps working
	//      (proxy routes were registered before this point so the cookie
	//      middleware doesn't apply to them — correct, Better Auth handles
	//      its own auth)
	//   3. AuthService.Middleware() (here) sets userAccountID from cookie
	//   4. /auth/* routes (here) — get the cookie middleware
	//   5. m.getGoogleAuthMiddleware (below) — applies to all subsequent
	//      routes; skips JWT resolution when cookie middleware already set
	if m.AuthService != nil {
		engine.Use(m.AuthService.Middleware())
		m.AuthService.RegisterRoutes(engine)
	}

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

	engine.POST("/rebalance", m.rebalance)
	engine.POST("/updateOrders", m.updateOrders)
	engine.POST("/sendSavedStrategySummaryEmails", m.sendSavedStrategySummaryEmails)

	return engine
}

func (m ApiHandler) StartApi(ctx context.Context) error {
	engine := m.InitializeRouterEngine(ctx)
	return engine.Run(fmt.Sprintf(":%d", m.Port))
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
	} else {
		lg = lg.With("requestID", req.RequestID.String())
		ctx.Set(logger.ContextKey, lg)
	}

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
	// Cookie-based auth (the new internal/auth package) runs as an earlier
	// middleware and may have already set userAccountID. When that's the
	// case we skip JWT/Bearer resolution entirely — cookie wins. This also
	// avoids accidentally swapping the identity if the FE sends both a
	// cookie and a stale Bearer token during the cutover from Better Auth.
	if v, exists := c.Get("userAccountID"); exists {
		if s, ok := v.(string); ok && s != "" {
			c.Next()
			return
		}
	}
	jwtStr := c.GetHeader("Authorization")
	if jwtStr == "" {
		c.Next()
		return
	}
	if !strings.HasPrefix(jwtStr, "Bearer ") {
		returnErrorJsonCode(fmt.Errorf("misformatted auth"), c, 403)
		return
	}
	jwtStr = jwtStr[len("Bearer "):]

	var userInput *model.UserAccount

	// Resolution order during the Supabase -> Better Auth cutover:
	//   1. Better Auth JWT (EdDSA via JWKS) - the new default
	//   2. Supabase JWT (HS256 / ES256 via JWKS) - kept until existing sessions expire
	//   3. Google ID token - used by older direct integrations
	// Once Supabase is fully decommissioned, branches 2 and the related
	// SupabaseJWT code in api/auth.go can be removed.
	var betterAuthErr error
	if m.BetterAuthJwksURL != "" {
		var baJwt *BetterAuthJWT
		baJwt, betterAuthErr = parseBetterAuthJWT(jwtStr, m.BetterAuthJwksURL, m.BetterAuthExpectedIssuer)
		if betterAuthErr == nil {
			userInput = &model.UserAccount{
				Provider:    model.UserAccountProviderType_BetterAuth,
				ProviderID:  &baJwt.Subject,
				PhoneNumber: baJwt.PhoneNumber,
			}
			if baJwt.Email != nil && *baJwt.Email != "" {
				userInput.Email = baJwt.Email
			}
			if baJwt.Name != "" {
				splits := strings.Split(baJwt.Name, " ")
				userInput.FirstName = &splits[0]
				if len(splits) > 1 {
					userInput.LastName = util.StringPointer(strings.Join(splits[1:], " "))
				}
			}
		}
	}

	var supabaseErr error
	var googleAuthErr error
	if userInput == nil {
		var parsedJwt *SupabaseJWT
		parsedJwt, supabaseErr = parseSupabaseJWT(jwtStr, m.JwtDecodeToken)
		userDetails, gErr := googleauth.GetUserDetails(jwtStr)
		googleAuthErr = gErr

		if supabaseErr == nil {
			userInput = &model.UserAccount{
				PhoneNumber: parsedJwt.PhoneNumber,
				Provider:    model.UserAccountProviderType_Supabase,
				ProviderID:  &parsedJwt.Subject,
			}
			if parsedJwt.Email != nil && *parsedJwt.Email != "" {
				userInput.Email = parsedJwt.Email
			}
			if parsedJwt.Name != "" {
				splits := strings.Split(parsedJwt.Name, " ")
				userInput.FirstName = &splits[0]
				if len(splits) > 1 {
					userInput.LastName = util.StringPointer(strings.Join(splits[1:], " "))
				}
			}
		} else if googleAuthErr == nil {
			userInput = &model.UserAccount{
				Email:     &userDetails.Email,
				FirstName: &userDetails.FirstName,
				LastName:  &userDetails.LastName,
				Provider:  model.UserAccountProviderType_Google,
			}
		}
	}

	if userInput == nil {
		// Log per-validator errors server-side for ops debugging; return a
		// generic 403 to the client so we don't expose token-validation
		// internals to attackers tuning forged tokens.
		logger.FromContext(c).With(
			"better_auth_err", fmt.Sprintf("%v", betterAuthErr),
			"supabase_err", fmt.Sprintf("%v", supabaseErr),
			"google_err", fmt.Sprintf("%v", googleAuthErr),
		).Warn("all auth methods failed")
		returnErrorJsonCode(fmt.Errorf("invalid or expired credentials"), c, 403)
		return
	}

	user, err := m.UserAccountRepository.GetOrCreate(userInput)
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

func (m ApiHandler) requireAuthenticatedUser(c *gin.Context) {
	if _, ok := c.Get("userAccountID"); !ok {
		returnErrorJsonCode(fmt.Errorf("authentication required"), c, 401)
		return
	}
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
