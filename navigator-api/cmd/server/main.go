package main

import (
	"log"
	"net/http"
	"time"

	"navigator-api/internal/config"
	"navigator-api/internal/database"
	"navigator-api/internal/handlers"
	"navigator-api/internal/middleware"
	"navigator-api/internal/services"
	"navigator-api/internal/worker"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("Starting NAVigator Go Backend API...")

	// 1. Load configuration
	cfg := config.Load()

	// 2. Initialize Database
	db, err := database.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 3. Initialize Services
	intSvc := services.NewIntegrationService(db)
	amfiSvc := services.NewAmfiService(intSvc)
	authSvc := services.NewAuthService(db, cfg)
	fundSvc := services.NewFundService(db, amfiSvc)
	caSvc := services.NewCorporateActionService(db)
	navSvc := services.NewNavService(db, amfiSvc, caSvc)
	simSvc := services.NewPortfolioSimulationService(db, navSvc, caSvc, fundSvc)
	userPortSvc := services.NewUserPortfolioService(db, navSvc)

	// 4. Initialize Handlers
	authHandler := handlers.NewAuthHandler(authSvc)
	fundHandler := handlers.NewFundHandler(fundSvc)
	navHandler := handlers.NewNavHandler(navSvc)
	portHandler := handlers.NewPortfolioHandler(simSvc)
	userPortHandler := handlers.NewUserPortfolioHandler(userPortSvc, db)
	caHandler := handlers.NewCorporateActionHandler(caSvc)
	adminHandler := handlers.NewAdminHandler(db, fundSvc, navSvc)
	intHandler := handlers.NewIntegrationHandler(intSvc)

	// 5. Start background NAV sync worker
	navWorker := worker.NewNavSyncWorker(db, amfiSvc, 6*time.Hour)
	navWorker.Start()
	defer navWorker.Stop()

	// 6. Setup Gin Router
	r := gin.Default()
	r.Use(middleware.CorsMiddleware(cfg))

	// Healthcheck
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "NAVigator API (Go)", "timestamp": time.Now().UTC()})
	})

	api := r.Group("/api")
	{
		// ─── Auth ───
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.GET("/me", middleware.AuthRequired(cfg), authHandler.Me)
			auth.GET("/users", middleware.AuthRequired(cfg), middleware.RequireRole("Admin"), authHandler.GetUsers)
			auth.PUT("/users/:userId/role", middleware.AuthRequired(cfg), middleware.RequireRole("Admin"), authHandler.UpdateUserRole)
		}

		// ─── Funds ───
		funds := api.Group("/funds")
		{
			funds.GET("", fundHandler.GetAll)
			funds.GET("/search", fundHandler.Search)
			funds.GET("/count", fundHandler.GetCount)
			funds.POST("/sync", middleware.AuthRequired(cfg), middleware.RequireRole("Admin"), fundHandler.Sync)
			funds.GET("/:schemeCode", fundHandler.GetBySchemeCode)
		}

		// ─── NAV ───
		nav := api.Group("/nav")
		{
			nav.GET("/gaps", middleware.AuthRequired(cfg), middleware.RequireRole("Admin"), navHandler.DetectAllGaps)
			nav.GET("/:schemeCode", navHandler.GetNavHistory)
			nav.GET("/:schemeCode/returns", navHandler.GetReturns)
			nav.GET("/:schemeCode/rolling-returns", navHandler.GetRollingReturns)
			nav.GET("/:schemeCode/gaps", middleware.AuthRequired(cfg), middleware.RequireRole("Admin"), navHandler.DetectGaps)
			nav.POST("/:schemeCode/repair-gaps", middleware.AuthRequired(cfg), middleware.RequireRole("Admin"), navHandler.RepairGaps)
		}

		// ─── Portfolio Simulation & Lab ───
		portfolio := api.Group("/portfolio")
		{
			portfolio.POST("/simulate", portHandler.Simulate)
			portfolio.POST("/compare", portHandler.Compare)
			portfolio.POST("/what-if", portHandler.WhatIf)
			portfolio.POST("/rolling-comparison", portHandler.RollingComparison)
		}

		// ─── User Portfolios ───
		userPort := api.Group("/user-portfolios")
		{
			userPort.POST("/fund-matches", userPortHandler.MatchFunds)
			userPort.GET("/grid-preferences/:gridKey", middleware.AuthRequired(cfg), userPortHandler.GetGridPreference)
			userPort.PUT("/grid-preferences/:gridKey", middleware.AuthRequired(cfg), userPortHandler.SaveGridPreference)

			userPort.GET("", middleware.AuthRequired(cfg), userPortHandler.GetPortfolios)
			userPort.POST("", middleware.AuthRequired(cfg), userPortHandler.CreatePortfolio)
			userPort.GET("/:portfolioId/dashboard", middleware.AuthRequired(cfg), userPortHandler.GetDashboard)
			userPort.GET("/:portfolioId/transactions", middleware.AuthRequired(cfg), userPortHandler.GetTransactions)
			userPort.PUT("/:portfolioId/transactions", middleware.AuthRequired(cfg), userPortHandler.SaveTransactions)
			userPort.POST("/:portfolioId/transactions/import-preview", middleware.AuthRequired(cfg), userPortHandler.ImportPreview)
			userPort.POST("/:portfolioId/transactions/import-commit", middleware.AuthRequired(cfg), userPortHandler.ImportCommit)
			userPort.POST("/:portfolioId/transactions/import-excel", middleware.AuthRequired(cfg), userPortHandler.ImportTransactions)
			userPort.GET("/:portfolioId/transactions/import-delete-preview", middleware.AuthRequired(cfg), userPortHandler.GetImportDeletePreview)
			userPort.POST("/:portfolioId/benchmark", middleware.AuthRequired(cfg), userPortHandler.CompareBenchmark)
		}

		// ─── Corporate Actions (both casing routes supported) ───
		registerCaRoutes := func(rg *gin.RouterGroup) {
			rg.Use(middleware.AuthRequired(cfg), middleware.RequireRole("Admin"))
			rg.GET("/anomalies", caHandler.DetectAnomalies)
			rg.GET("/review-queue", caHandler.GetReviewQueue)
			rg.POST("/auto-record-anomalies", caHandler.AutoRecordAnomalies)
			rg.GET("/actions/:schemeCode", caHandler.GetActions)
			rg.POST("", caHandler.CreateAction)
			rg.PUT("/:id", caHandler.UpdateAction)
			rg.DELETE("/:id", caHandler.DeleteAction)
		}
		registerCaRoutes(api.Group("/corporateactions"))
		registerCaRoutes(api.Group("/corporate-actions"))

		// ─── Admin ───
		admin := api.Group("/admin")
		admin.Use(middleware.AuthRequired(cfg), middleware.RequireRole("Admin"))
		{
			admin.GET("/funds/summary", adminHandler.GetFundsSummary)
			admin.GET("/funds", adminHandler.GetFunds)
			admin.POST("/funds/sync", adminHandler.SyncFundMaster)
			admin.POST("/funds/:schemeCode/nav-history/sync", adminHandler.SyncFundNavHistory)
			admin.DELETE("/funds/:schemeCode/nav-history", adminHandler.DeleteFundNavHistory)
			admin.DELETE("/funds/:schemeCode", adminHandler.DeleteFund)
			admin.DELETE("/funds/all", adminHandler.DeleteAllFunds)

			// Integrations & Health Center
			admin.GET("/integrations", intHandler.GetEndpoints)
			admin.PUT("/integrations/:key", intHandler.UpdateEndpoint)
			admin.POST("/integrations/:key/reset", intHandler.ResetEndpoint)
			admin.POST("/integrations/:key/test", intHandler.TestEndpoint)
			admin.POST("/integrations/health-all", intHandler.HealthCheckAll)
		}
	}

	addr := ":" + cfg.Port
	log.Printf("Server listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
