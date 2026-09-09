package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// Blank-imported so the IANA timezone database is embedded directly
	// into the compiled binary, rather than relying on the OS having it
	// installed at /usr/share/zoneinfo. Railway's (and many minimal
	// container images') deployment environment does NOT include this
	// by default — without this import, every time.LoadLocation call
	// anywhere in the app (the backup scheduler and its Phnom Penh
	// timestamps, the audit log retention cutoffs) fails with "unknown
	// time zone", regardless of how correct the calling code is.
	_ "time/tzdata"

	"bubblewhite-backend/config"
	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/routes"
	"bubblewhite-backend/seed"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()

	db := config.ConnectDatabase(cfg)
	config.AutoMigrate(db)

	config.ConnectR2(cfg)

	seed.Run(db)

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(middlewares.CORSMiddleware(cfg))

	// Caps every request body at 15MB — comfortably above a real product
	// photo upload (the largest legitimate payload this API accepts;
	// every JSON API call is only ever a few KB), while preventing an
	// attacker from sending an arbitrarily large payload to exhaust
	// server memory or disk.
	r.Use(middlewares.MaxBodySize(15 << 20))

	// General-purpose, generous global rate limit — 300/minute with
	// bursts up to 60, comfortably above what a real user could ever
	// trigger (a single page load pulling in several API calls at once
	// is nowhere near this), but low enough to meaningfully slow down a
	// scripted flood. Skips the PPCBank webhook path — see
	// RateLimiter.SkipPaths' own comment for why that endpoint needs to
	// be exempt rather than just given a higher number.
	//
	// Also skips /api/admin/ — every admin route already sits behind its
	// own dedicated AdminRateLimiter (600/min, burst 120 — see routes.go),
	// deliberately more generous than this global limit since real admin
	// work is bursty in a way ordinary browsing isn't. Without this skip,
	// that more generous limiter would be silently pointless: both
	// middlewares run on every admin request, and the STRICTER one always
	// wins — an admin doing bulk work would still get capped at this
	// limiter's 300/min regardless of the 600/min ceiling meant to cover
	// them, since this one runs first and rejects before AdminRateLimiter
	// is ever reached.
	globalLimiter := middlewares.NewRateLimiter(300, 60, "សំណើច្រើនពេក សូមព្យាយាមម្តងទៀតក្នុងពេលឆាប់ៗនេះ។").
		Name("global").
		SkipPaths("/api/webhooks/", "/api/admin/").
		Allowlist(cfg.RateLimitAllowlist...)
	r.Use(globalLimiter.Middleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Swagger UI + raw OpenAPI spec (see docs/).
	r.Static("/docs", "./docs")

	container := routes.Build(db)
	routes.RegisterRoutes(r, container)

	// Daily database backup to Telegram — see BackupService's own doc
	// comment for why this is a background goroutine rather than a
	// separate cron job/process, and for the deliberate choice to
	// generate the dump in pure Go rather than shelling out to mysqldump.
	container.BackupService.StartScheduler()

	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	// A custom http.Server with explicit timeouts, rather than Gin's bare
	// r.Run() — that convenience wrapper uses Go's http.Server defaults,
	// which is genuinely zero timeout on ReadHeaderTimeout/ReadTimeout/
	// WriteTimeout/IdleTimeout. A client that opens a connection and
	// sends data (or nothing at all) extremely slowly can hold that
	// connection open indefinitely under the defaults — the classic
	// "Slowloris" attack, where enough such connections exhaust the
	// server's available connections without ever sending a real flood
	// of traffic. ReadHeaderTimeout is the main defense here: headers
	// should always arrive within a few seconds regardless of what the
	// request is about, even on a slow connection. WriteTimeout is set
	// higher than PPCBank's own 15s client timeout (see
	// services/ppcbank_payment.go's httpClient) with a buffer, since a
	// checkout request legitimately waits on that external call before
	// this server ever writes a response back to the customer — too
	// short a WriteTimeout here would cut off a real, in-progress
	// checkout, not just an attacker.
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("BubbleWhite API listening on :%s (env=%s)", port, cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown — needed now that main() manages the server
	// directly instead of letting r.Run() block forever; without this,
	// a deploy/restart would kill in-flight requests mid-response
	// instead of letting them finish first.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
}
