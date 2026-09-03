package routes

import (
	"bubblewhite-backend/controllers"
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
	PaymentMethod  *controllers.PaymentMethodController
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

	// Controllers
	return &Container{
		DB: db,

		Auth:           controllers.NewAuthController(authService),
		Product:        controllers.NewProductController(productService),
		Category:       controllers.NewCategoryController(categoryService),
		User:           controllers.NewUserController(userService, roleService),
		Role:           controllers.NewRoleController(roleService, db),
		Settings:       controllers.NewSettingsController(settingsService),
		PaymentMethod:  controllers.NewPaymentMethodController(paymentMethodService),
		Contact:        controllers.NewContactController(contactService),
		Upload:         controllers.NewUploadController(uploadService),
		Banner:         controllers.NewBannerController(bannerService),
		Customer:       controllers.NewCustomerController(customerService),
		Cart:           controllers.NewCartController(cartService),
		Order:          controllers.NewOrderController(orderService),
		AdminOrder:     controllers.NewAdminOrderController(orderService),
		AdminCustomer:  controllers.NewAdminCustomerController(customerService),
		PPCBankWebhook: controllers.NewPPCBankWebhookController(orderService),
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
	RegisterWebhookRoutes(api, c)
	RegisterAdminOrderRoutes(api, c)
}
