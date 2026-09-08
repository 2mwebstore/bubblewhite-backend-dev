package routes

import (
	"bubblewhite-backend/config"
	"bubblewhite-backend/controllers"
	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Container holds every wired controller, so route files can pull what they
// need without re-doing dependency injection themselves.
type Container struct {
	DB *gorm.DB

	Auth           *controllers.AuthController
	Product        *controllers.ProductController
	Category       *controllers.CategoryController
	User           *controllers.UserController
	Role           *controllers.RoleController
	Settings       *controllers.SettingsController
	Contact        *controllers.ContactController
	Upload         *controllers.UploadController
	Banner         *controllers.BannerController
	Customer       *controllers.CustomerController
	Cart           *controllers.CartController
	Order          *controllers.OrderController
	AdminOrder     *controllers.AdminOrderController
	AdminCustomer  *controllers.AdminCustomerController
	PPCBankWebhook *controllers.PPCBankWebhookController
	TelegramBot    *controllers.TelegramBotController
	AuditLog       *controllers.AdminAuditLogController
	PaymentMethod  *controllers.PaymentMethodController

	// Stricter, dedicated rate limiters for endpoints that are actual
	// abuse targets in a way general browsing isn't — repeated login
	// attempts (credential stuffing/brute force) and repeated form
	// submissions (spam) — kept separate from the global limiter applied
	// to every route, which is deliberately generous since it has to
	// cover legitimate browsing/shopping traffic too.
	LoginRateLimiter   *middlewares.RateLimiter
	ContactRateLimiter *middlewares.RateLimiter
	// AdminRateLimiter sits on every /admin/** route group, on top of the
	// global limiter — a second, admin-specific ceiling so a single
	// compromised staff token or a runaway script can't hammer the API
	// indefinitely just because it's authenticated. Deliberately more
	// generous than the global limiter (real admin work — bulk product
	// edits, CSV-style imports done one row at a time, paging through a
	// large order list) is bursty in a way ordinary storefront browsing
	// isn't, and admins are a small, known set of people, not the general
	// public the global limiter has to stay conservative for.
	AdminRateLimiter *middlewares.RateLimiter
	// CheckoutRateLimiter targets order creation specifically — a genuine
	// high-value target (spamming orders to exhaust inventory, or
	// repeatedly re-initiating PPCBank payment sessions) that the global
	// limiter alone doesn't call out. A real customer checks out once,
	// maybe retries a couple of times if a payment attempt fails.
	CheckoutRateLimiter *middlewares.RateLimiter
}

// Build wires repositories -> services -> controllers. This is the single
// place that knows how the whole dependency graph fits together.
func Build(db *gorm.DB) *Container {
	services.SetPPCBankDB(db)

	// Repositories
	userRepo := repositories.NewUserRepository(db)
	roleRepo := repositories.NewRoleRepository(db)
	productRepo := repositories.NewProductRepository(db)
	categoryRepo := repositories.NewCategoryRepository(db)
	contactRepo := repositories.NewContactRepository(db)
	settingsRepo := repositories.NewSettingsRepository(db)
	paymentMethodRepo := repositories.NewPaymentMethodRepository(db)
	bannerRepo := repositories.NewBannerRepository(db)
	customerRepo := repositories.NewCustomerRepository(db)
	cartRepo := repositories.NewCartRepository(db)
	orderRepo := repositories.NewOrderRepository(db)

	// Services
	authService := services.NewAuthService(userRepo)
	productService := services.NewProductService(productRepo)
	categoryService := services.NewCategoryService(categoryRepo)
	userService := services.NewUserService(userRepo)
	roleService := services.NewRoleService(roleRepo)
	settingsService := services.NewSettingsService(settingsRepo)
	paymentMethodService := services.NewPaymentMethodService(paymentMethodRepo)
	contactService := services.NewContactService(contactRepo)
	uploadService := services.NewUploadService()
	bannerService := services.NewBannerService(bannerRepo)
	customerService := services.NewCustomerService(customerRepo)
	cartService := services.NewCartService(cartRepo, productRepo)
	orderService := services.NewOrderService(orderRepo, cartRepo, productRepo, paymentMethodService, settingsRepo)
	googleOAuthService := services.NewGoogleOAuthService(config.Get().GoogleClientID)
	facebookOAuthService := services.NewFacebookOAuthService(config.Get().FacebookAppID, config.Get().FacebookAppSecret)

	auditLogRepo := repositories.NewAuditLogRepository(db)
	auditLogService := services.NewAuditLogService(auditLogRepo)

	otpRepo := repositories.NewOtpRepository(db)
	telegramLinkRepo := repositories.NewTelegramPhoneLinkRepository(db)
	plasgateSMS := services.NewPlasgateSMSService(config.Get().PlasgatePrivateKey, config.Get().PlasgateSecretKey, config.Get().PlasgateSenderID)
	otpService := services.NewOtpService(otpRepo, telegramLinkRepo, plasgateSMS)

	// Controllers
	return &Container{
		DB: db,

		Auth:           controllers.NewAuthController(authService, auditLogService),
		Product:        controllers.NewProductController(productService, auditLogService),
		Category:       controllers.NewCategoryController(categoryService, auditLogService),
		User:           controllers.NewUserController(userService, roleService, auditLogService),
		Role:           controllers.NewRoleController(roleService, db, auditLogService),
		Settings:       controllers.NewSettingsController(settingsService, auditLogService),
		PaymentMethod:  controllers.NewPaymentMethodController(paymentMethodService, auditLogService),
		Contact:        controllers.NewContactController(contactService),
		Upload:         controllers.NewUploadController(uploadService),
		Banner:         controllers.NewBannerController(bannerService, auditLogService),
		Customer:       controllers.NewCustomerController(customerService, googleOAuthService, facebookOAuthService, otpService, auditLogService),
		Cart:           controllers.NewCartController(cartService),
		Order:          controllers.NewOrderController(orderService, auditLogService),
		AdminOrder:     controllers.NewAdminOrderController(orderService, auditLogService),
		AdminCustomer:  controllers.NewAdminCustomerController(customerService, auditLogService),
		PPCBankWebhook: controllers.NewPPCBankWebhookController(orderService),
		TelegramBot:    controllers.NewTelegramBotController(otpService),
		AuditLog:       controllers.NewAdminAuditLogController(auditLogService),

		// 6/minute allows a few genuine mistyped-password retries without
		// friction, while still shutting down a sustained brute-force
		// attempt long before it could work through any real password space.
		LoginRateLimiter: middlewares.NewRateLimiter(6, 6, "ការព្យាយាមចូលច្រើនពេក សូមរង់ចាំបន្តិច។").
			Name("login").
			Allowlist(config.Get().RateLimitAllowlist...),
		// 3/minute — a real visitor submits this form once, maybe twice
		// if they made a typo; this is squarely aimed at scripted spam.
		ContactRateLimiter: middlewares.NewRateLimiter(3, 3, "សូមរង់ចាំបន្តិចមុននឹងផ្ញើសារម្តងទៀត។").
			Name("contact").
			Allowlist(config.Get().RateLimitAllowlist...),
		// 600/minute, burst 120 — well above the global limiter's 300,
		// deliberately: real admin work (bulk edits, paging a large list,
		// uploading several images in a row) is bursty in a way ordinary
		// storefront browsing isn't, and this only ever applies to a
		// small, known set of staff accounts, not the general public.
		// Still a real ceiling — a compromised token or a runaway script
		// hitting the API in a tight loop gets capped, not left
		// unbounded just because it's authenticated.
		AdminRateLimiter: middlewares.NewRateLimiter(600, 120, "សំណើច្រើនពេក សូមរង់ចាំបន្តិច។").
			Name("admin").
			Allowlist(config.Get().RateLimitAllowlist...),
		// 10/minute — a real customer checks out once per visit, maybe a
		// couple of times if PPCBank's redirect flow needs retrying; this
		// targets scripted order spam, not normal shopping.
		CheckoutRateLimiter: middlewares.NewRateLimiter(10, 10, "សូមរង់ចាំបន្តិចមុននឹងព្យាយាមម្តងទៀត។").
			Name("checkout").
			Allowlist(config.Get().RateLimitAllowlist...),
	}
}

// RegisterRoutes mounts every route group onto the given Gin engine.
func RegisterRoutes(r *gin.Engine, c *Container) {
	api := r.Group("/api")

	RegisterAuthRoutes(api, c)
	RegisterProductRoutes(api, c)
	RegisterCategoryRoutes(api, c)
	RegisterSettingsRoutes(api, c)
	RegisterPaymentMethodRoutes(api, c)
	RegisterContactRoutes(api, c)
	RegisterUserRoutes(api, c)
	RegisterRoleRoutes(api, c)
	RegisterUploadRoutes(api, c)
	RegisterBannerRoutes(api, c)
	RegisterCustomerRoutes(api, c)
	RegisterOrderRoutes(api, c)
	RegisterAdminCustomerRoutes(api, c)
	RegisterAuditLogRoutes(api, c)
	RegisterWebhookRoutes(api, c)
	RegisterAdminOrderRoutes(api, c)
}
