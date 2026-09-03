package seed

import (
	"log"

	"bubblewhite-backend/config"
	"bubblewhite-backend/models"
	"bubblewhite-backend/utils"

	"gorm.io/gorm"
)

// PermissionCatalog is the fixed list of every grantable permission in the
// system. It's code, not data, so it can't drift out of sync with what the
// controllers actually check via middlewares.RequirePermission(...).
var PermissionCatalog = []models.Permission{
	{Slug: "product.view", Group: "product", Description: "View products"},
	{Slug: "product.create", Group: "product", Description: "Create products"},
	{Slug: "product.update", Group: "product", Description: "Update products (incl. uploading images)"},
	{Slug: "product.delete", Group: "product", Description: "Delete products"},

	{Slug: "category.view", Group: "category", Description: "View categories"},
	{Slug: "category.create", Group: "category", Description: "Create categories"},
	{Slug: "category.update", Group: "category", Description: "Update categories (incl. uploading images)"},
	{Slug: "category.delete", Group: "category", Description: "Delete categories"},

	{Slug: "contact.view", Group: "contact", Description: "View contact form submissions"},
	{Slug: "contact.manage", Group: "contact", Description: "Mark read / delete contact messages"},

	{Slug: "settings.view", Group: "settings", Description: "View company/contact settings"},
	{Slug: "settings.update", Group: "settings", Description: "Update company/contact settings and logo"},

	{Slug: "payment_method.view", Group: "payment_method", Description: "View payment methods"},
	{Slug: "payment_method.update", Group: "payment_method", Description: "Enable/disable, reorder, set default, and upload logos for payment methods"},

	{Slug: "user.view", Group: "user", Description: "View admin users"},
	{Slug: "user.create", Group: "user", Description: "Create admin users"},
	{Slug: "user.update", Group: "user", Description: "Update admin users"},
	{Slug: "user.delete", Group: "user", Description: "Delete admin users"},
	{Slug: "user.manage", Group: "user", Description: "Assign roles to admin users"},

	{Slug: "role.view", Group: "role", Description: "View roles and permissions"},
	{Slug: "role.create", Group: "role", Description: "Create roles"},
	{Slug: "role.update", Group: "role", Description: "Update a role's permissions"},
	{Slug: "role.delete", Group: "role", Description: "Delete roles"},

	{Slug: "banner.view", Group: "banner", Description: "View home page banners"},
	{Slug: "banner.create", Group: "banner", Description: "Upload/add home page banners"},
	{Slug: "banner.update", Group: "banner", Description: "Update or reorder home page banners"},
	{Slug: "banner.delete", Group: "banner", Description: "Delete home page banners"},

	{Slug: "order.view", Group: "order", Description: "View customer orders"},
	{Slug: "order.manage", Group: "order", Description: "Update order status"},

	{Slug: "customer.view", Group: "customer", Description: "View storefront customers"},
	{Slug: "customer.manage", Group: "customer", Description: "Reset customer passwords, activate/deactivate accounts"},
}

func allPermissionSlugs() []string {
	slugs := make([]string, len(PermissionCatalog))
	for i, p := range PermissionCatalog {
		slugs[i] = p.Slug
	}
	return slugs
}

// stringSlicesEqualUnordered reports whether two slices contain the same
// set of strings, ignoring order — used to detect whether Administrator's
// stored permission list actually needs re-saving, rather than writing to
// the database on every single boot regardless of whether anything changed.
func stringSlicesEqualUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		if !set[s] {
			return false
		}
	}
	return true
}

// Run performs all first-boot seeding. Each step is individually guarded so
// re-running on an already-seeded database is always a safe no-op.
func Run(db *gorm.DB) {
	seedPermissions(db)
	adminRole := seedRoles(db)
	seedAdminUser(db, adminRole)
	seedSettings(db)
	seedPaymentMethods(db)
	seedCatalog(db)
	seedBanners(db)
}

// seedPermissions upserts the permission catalog by slug, so adding a new
// permission to the Go slice above and restarting is enough to make it
// available to assign to roles.
func seedPermissions(db *gorm.DB) {
	for _, p := range PermissionCatalog {
		var existing models.Permission
		err := db.Where("slug = ?", p.Slug).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := db.Create(&p).Error; err != nil {
				log.Printf("seed: failed to create permission %s: %v", p.Slug, err)
			}
		}
	}
}

// seedRoles creates the two built-in roles if they don't exist yet, and
// returns the admin role (needed to attach the seed admin user to it).
func seedRoles(db *gorm.DB) models.Role {
	allSlugs := allPermissionSlugs()

	var admin models.Role
	err := db.Where("slug = ?", "admin").First(&admin).Error
	if err == gorm.ErrRecordNotFound {
		admin = models.Role{
			Name:        "Administrator",
			Slug:        "admin",
			Description: "Full access to everything, including managing other admin users and roles.",
			Permissions: models.JSONColumn[[]string]{Data: allSlugs},
			IsSystem:    true,
		}
		if err := db.Create(&admin).Error; err != nil {
			log.Printf("seed: failed to create admin role: %v", err)
		} else {
			log.Println("seed: created 'admin' role")
		}
	} else if err == nil {
		// Self-healing: re-sync the Administrator role's permission list
		// with the full catalog on EVERY boot, not just at first creation.
		// Administrator holds a real, enumerated list of every permission
		// (not a "*" wildcard shorthand), so this is what keeps that list
		// from silently going stale every time PermissionCatalog above
		// gains a new entry — without it, adding a permission would
		// require someone to remember to manually re-grant it to
		// Administrator, which is exactly the kind of gap that quietly
		// locks the admin out of new features until someone notices.
		if !stringSlicesEqualUnordered(admin.Permissions.Data, allSlugs) {
			admin.Permissions = models.JSONColumn[[]string]{Data: allSlugs}
			if err := db.Save(&admin).Error; err != nil {
				log.Printf("seed: failed to sync admin role permissions: %v", err)
			} else {
				log.Println("seed: synced 'admin' role permissions with the current catalog")
			}
		}
	}

	var editor models.Role
	err = db.Where("slug = ?", "editor").First(&editor).Error
	if err == gorm.ErrRecordNotFound {
		editor = models.Role{
			Name:        "Editor",
			Slug:        "editor",
			Description: "Manages products, categories, banners, and contact messages — no access to users, roles, or settings.",
			Permissions: models.JSONColumn[[]string]{Data: []string{
				"product.view", "product.create", "product.update", "product.delete",
				"category.view", "category.create", "category.update", "category.delete",
				"banner.view", "banner.create", "banner.update", "banner.delete",
				"contact.view", "contact.manage",
			}},
			IsSystem: true,
		}
		if err := db.Create(&editor).Error; err != nil {
			log.Printf("seed: failed to create editor role: %v", err)
		} else {
			log.Println("seed: created 'editor' role")
		}
	}

	return admin
}

// seedAdminUser creates the first login (from SEED_ADMIN_EMAIL/PASSWORD)
// if no users exist yet at all.
func seedAdminUser(db *gorm.DB, adminRole models.Role) {
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count > 0 {
		return
	}

	cfg := config.Get()
	hash, err := utils.HashPassword(cfg.SeedAdminPassword)
	if err != nil {
		log.Printf("seed: failed to hash admin password: %v", err)
		return
	}

	admin := models.User{
		Name:         "Admin",
		Email:        cfg.SeedAdminEmail,
		PasswordHash: hash,
		RoleID:       adminRole.ID,
		IsActive:     true,
	}
	if err := db.Create(&admin).Error; err != nil {
		log.Printf("seed: failed to create admin user: %v", err)
		return
	}
	log.Printf("seed: created admin user %s — change the seeded password after first login", cfg.SeedAdminEmail)
}

// seedSettings creates the single settings row with starter values if it
// doesn't exist yet. Everything here is editable from the admin panel afterward.
func seedSettings(db *gorm.DB) {
	var count int64
	db.Model(&models.Settings{}).Count(&count)
	if count > 0 {
		return
	}

	settings := models.Settings{
		ID:             models.SettingsID,
		CompanyName:    "Bubble White",
		CompanyDetail:  "Bubble White ជាម៉ាកសម្លៀកបំពាក់នៅភ្នំពេញ ដែលផ្តល់នូវសម្លៀកបំពាក់ប្រចាំថ្ងៃដ៏សាមញ្ញ និងសុខស្រួល។",
		ContactEmail:   "hello@bubblewhite.co",
		ContactPhone:   "+855 12 345 678",
		ContactAddress: "ភ្នំពេញ, កម្ពុជា",
		WorkingHours:   "ចន្ទ – អាទិត្យ / ៩ព្រឹក – ៩យប់",
	}
	if err := db.Create(&settings).Error; err != nil {
		log.Printf("seed: failed to create settings row: %v", err)
		return
	}
	log.Println("seed: created default settings row")
}

// seedPaymentMethods creates the built-in payment method rows if they
// don't exist yet, migrating their initial Enabled state from the
// existing Settings.*PaymentEnabled booleans (if a settings row already
// exists) so nobody's current configuration gets silently reset by this
// migration to a proper table. IsPrimary defaults to PPCBank, the primary
// digital payment option now that Bakong's integration has been removed.
func seedPaymentMethods(db *gorm.DB) {
	var count int64
	db.Model(&models.PaymentMethod{}).Count(&count)
	if count > 0 {
		return
	}

	var settings models.Settings
	hasSettings := db.First(&settings, models.SettingsID).Error == nil

	cashEnabled := true
	ppcbankEnabled := false
	if hasSettings {
		cashEnabled = settings.CashPaymentEnabled
		ppcbankEnabled = settings.PPCBankPaymentEnabled
	}

	methods := []models.PaymentMethod{
		{Code: models.PaymentMethodPPCBank, Name: "PPCBank KHQR", Enabled: ppcbankEnabled, IsPrimary: true, SortOrder: 1},
		{Code: models.PaymentMethodCash, Name: "សាច់ប្រាក់", Enabled: cashEnabled, IsPrimary: false, SortOrder: 2},
	}
	if err := db.Create(&methods).Error; err != nil {
		log.Printf("seed: failed to create payment methods: %v", err)
		return
	}
	log.Println("seed: created payment method rows (migrated enabled state from existing settings, if any)")
}

// seedCatalog seeds starter categories + products matching the frontend's
// previous static data/products.js, so the API isn't empty on first boot.
func seedCatalog(db *gorm.DB) {
	var catCount int64
	db.Model(&models.Category{}).Count(&catCount)
	if catCount > 0 {
		return
	}

	log.Println("seed: seeding starter categories and products...")

	categories := []models.Category{
		{Name: "បុរស", Slug: "men", Image: "", SortOrder: 1},
		{Name: "នារី", Slug: "women", Image: "", SortOrder: 2},
		{Name: "អាវយឺត", Slug: "t-shirts", Image: "", SortOrder: 3},
		{Name: "អាវហ៊ូឌី", Slug: "hoodies", Image: "", SortOrder: 4},
		{Name: "គ្រឿងបន្លាស់", Slug: "accessories", Image: "", SortOrder: 5},
	}
	if err := db.Create(&categories).Error; err != nil {
		log.Printf("seed: failed to insert categories: %v", err)
	}

	badgeNew := "ថ្មី"
	badgeDiscount := "-10%"
	sweatshirtCompareAt := 39.99

	products := []models.Product{
		{
			ID: "bw-basic-tee", Name: "Bubble White អាវយឺតធម្មតា", Price: 19.99,
			CategoryID: "t-shirts", Badge: &badgeNew, Featured: false,
			Sizes:       models.JSONColumn[[]string]{Data: []string{"S", "M", "L", "XL"}},
			Images:      models.JSONColumn[[]string]{Data: []string{}},
			Description: "អាវយឺតប្រចាំថ្ងៃធ្វើពីកប្បាសក្រាស់ស្តើងសមរម្យ។ ស្មើមុត ទន់ភ្លន់ ទំហំធំល្មម និងស្លាកគំនូរដេរតូចមួយនៅទ្រូង។",
		},
		{
			ID: "bw-hoodie", Name: "Bubble White អាវហ៊ូឌី", Price: 39.99,
			CategoryID: "hoodies", Badge: &badgeNew, Featured: true,
			Sizes:       models.JSONColumn[[]string]{Data: []string{"S", "M", "L", "XL"}},
			Images:      models.JSONColumn[[]string]{Data: []string{}},
			Description: "អាវហ៊ូឌីរោមកម្រាស់មធ្យម មានពាក់ក្បាលមានស្រទាប់ក្នុង ស្មាទម្លាក់ និងស្លាកគំនូរនៅទ្រូង។",
		},
		{
			ID: "bw-oversize-tee", Name: "Bubble White អាវយឺតធំ", Price: 24.99,
			CategoryID: "t-shirts", Featured: true,
			Sizes:       models.JSONColumn[[]string]{Data: []string{"S", "M", "L", "XL"}},
			Images:      models.JSONColumn[[]string]{Data: []string{}},
			Description: "ម៉ូតធំទូលាយ ស្មាទម្លាក់ និងគំនូរដិតធំនៅមុខអាវ។",
		},
		{
			ID: "bw-sweatshirt", Name: "Bubble White អាវយឺតកក់ក្តៅ", Price: 35.99,
			CompareAt: &sweatshirtCompareAt, CategoryID: "men", Badge: &badgeDiscount, Featured: false,
			Sizes:       models.JSONColumn[[]string]{Data: []string{"S", "M", "L", "XL"}},
			Images:      models.JSONColumn[[]string]{Data: []string{}},
			Description: "អាវយឺតកក់ក្តៅក របូបខាងក្នុងទន់ភ្លន់។",
		},
		{
			ID: "bw-cap", Name: "Bubble White មួកកាប់", Price: 14.99,
			CategoryID: "accessories", Featured: true,
			Sizes:       models.JSONColumn[[]string]{Data: []string{"One Size"}},
			Images:      models.JSONColumn[[]string]{Data: []string{}},
			Description: "មួកកាប់ប្រាំមួយផ្ទាំង រចនាឡើងមានទម្រង់រឹងមាំ។",
		},
	}
	if err := db.Create(&products).Error; err != nil {
		log.Printf("seed: failed to insert products: %v", err)
	}
}

// seedBanners creates a couple of placeholder home-page banners on first
// boot so the hero carousel isn't empty — replace their imageUrl (and add
// more) from the admin panel's Banners page.
func seedBanners(db *gorm.DB) {
	var count int64
	db.Model(&models.Banner{}).Count(&count)
	if count > 0 {
		return
	}

	banners := []models.Banner{
		{ImageURL: "", Alt: "Bubble White — Minimal style, maximum comfort", SortOrder: 1, IsActive: false},
		{ImageURL: "", Alt: "Bubble White new collection", SortOrder: 2, IsActive: false},
	}
	if err := db.Create(&banners).Error; err != nil {
		log.Printf("seed: failed to insert banners: %v", err)
		return
	}
	log.Println("seed: created placeholder banners (inactive — set an image and activate them from the admin panel)")
}
