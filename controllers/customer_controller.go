package controllers

import (
	"errors"

	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/models"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type CustomerController struct {
	Service  *services.CustomerService
	Google   *services.GoogleOAuthService
	Facebook *services.FacebookOAuthService
}

func NewCustomerController(s *services.CustomerService, google *services.GoogleOAuthService, facebook *services.FacebookOAuthService) *CustomerController {
	return &CustomerController{Service: s, Google: google, Facebook: facebook}
}

// customerJSON builds a consistent response shape across register/login/me/
// update/admin-list — Phone and Email are *string on the model (nil when
// not set — Phone in particular is nil for a Google/Facebook signup that
// hasn't provided one at checkout yet), flattened here to empty strings so
// the frontend never has to special-case null.
func customerJSON(customer *models.Customer) gin.H {
	phone := ""
	if customer.Phone != nil {
		phone = *customer.Phone
	}
	email := ""
	if customer.Email != nil {
		email = *customer.Email
	}
	return gin.H{
		"id":        customer.ID,
		"name":      customer.Name,
		"phone":     phone,
		"email":     email,
		"isActive":  customer.IsActive,
		"createdAt": customer.CreatedAt,
	}
}

// customerTokenIdentifier picks whatever this customer actually has to
// identify them in CustomerClaims.Identifier — phone first (the original,
// primary identifier), falling back to email, and finally a placeholder
// for the rare case for a brand-new Google/Facebook signup with neither
// (Google always provides a verified email in practice, but Facebook's
// email field can genuinely be empty). This field is informational only
// (see utils.CustomerClaims's own comment) — nothing security-relevant
// depends on it never being empty.
func customerTokenIdentifier(customer *models.Customer) string {
	if customer.Phone != nil && *customer.Phone != "" {
		return *customer.Phone
	}
	if customer.Email != nil && *customer.Email != "" {
		return *customer.Email
	}
	return "customer"
}

type registerInput struct {
	Name     string `json:"name" validate:"required"`
	Phone    string `json:"phone" validate:"required"`
	Email    string `json:"email" validate:"omitempty,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// POST /api/customer/register — public
func (ctrl *CustomerController) Register(c *gin.Context) {
	var in registerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	customer, err := ctrl.Service.Register(in.Name, in.Phone, in.Email, in.Password)
	if err != nil {
		if errors.Is(err, services.ErrPhoneAlreadyRegistered) {
			utils.FailWithErrors(c, map[string]string{"phone": "លេខទូរស័ព្ទនេះបានចុះឈ្មោះរួចហើយ"})
			return
		}
		if errors.Is(err, services.ErrEmailAlreadyRegistered) {
			utils.FailWithErrors(c, map[string]string{"email": "អ៊ីមែលនេះបានចុះឈ្មោះរួចហើយ"})
			return
		}
		utils.InternalError(c, "failed to register")
		return
	}

	token, err := utils.SignCustomerToken(customer.ID, customerTokenIdentifier(customer))
	if err != nil {
		utils.InternalError(c, "failed to sign in after registration")
		return
	}

	utils.Created(c, gin.H{"token": token, "customer": customerJSON(customer)})
}

type customerLoginInput struct {
	Identifier string `json:"identifier" validate:"required"` // phone OR email
	Password   string `json:"password" validate:"required"`
}

// POST /api/customer/login — public. Accepts either a phone number or an
// email as the identifier, since email is optional at registration.
func (ctrl *CustomerController) Login(c *gin.Context) {
	var in customerLoginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	customer, err := ctrl.Service.Login(in.Identifier, in.Password)
	if err != nil {
		if errors.Is(err, services.ErrCustomerInactive) {
			utils.Forbidden(c, "គណនីនេះត្រូវបានផ្អាក សូមទាក់ទងមកយើង")
			return
		}
		if errors.Is(err, services.ErrOAuthAccountNoPassword) {
			utils.Unauthorized(c, "គណនីនេះប្រើការចូលតាម Google ឬ Facebook សូមចូលតាមវិធីនោះវិញ")
			return
		}
		utils.Unauthorized(c, "លេខទូរស័ព្ទ/អ៊ីមែល ឬពាក្យសម្ងាត់មិនត្រឹមត្រូវ")
		return
	}

	token, err := utils.SignCustomerToken(customer.ID, customerTokenIdentifier(customer))
	if err != nil {
		utils.InternalError(c, "failed to sign in")
		return
	}

	utils.OK(c, gin.H{"token": token, "customer": customerJSON(customer)})
}

type googleLoginInput struct {
	IDToken string `json:"idToken" validate:"required"`
}

// POST /api/customer/auth/google — public. Accepts the ID token Google's
// Identity Services JS library returns to the frontend after a successful
// sign-in, verifies it independently against Google's own public keys
// (never trusting the token's claims just because the frontend sent them —
// see GoogleOAuthService.VerifyIDToken), then finds-or-creates the
// matching customer and signs them in with the same token shape as every
// other login path.
func (ctrl *CustomerController) GoogleLogin(c *gin.Context) {
	var in googleLoginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	googleUser, err := ctrl.Google.VerifyIDToken(in.IDToken)
	if err != nil {
		if errors.Is(err, services.ErrGoogleNotConfigured) {
			utils.InternalError(c, "google sign-in is not available right now")
			return
		}
		utils.Unauthorized(c, "មិនអាចផ្ទៀងផ្ទាត់គណនី Google បានទេ")
		return
	}

	customer, err := ctrl.Service.LoginOrRegisterWithGoogle(googleUser.Sub, googleUser.Email, googleUser.Name)
	if err != nil {
		if errors.Is(err, services.ErrCustomerInactive) {
			utils.Forbidden(c, "គណនីនេះត្រូវបានផ្អាក សូមទាក់ទងមកយើង")
			return
		}
		utils.InternalError(c, "failed to sign in with google")
		return
	}

	token, err := utils.SignCustomerToken(customer.ID, customerTokenIdentifier(customer))
	if err != nil {
		utils.InternalError(c, "failed to sign in")
		return
	}
	utils.OK(c, gin.H{"token": token, "customer": customerJSON(customer)})
}

type facebookLoginInput struct {
	AccessToken string `json:"accessToken" validate:"required"`
}

// POST /api/customer/auth/facebook — public. Accepts the access token
// Facebook's JS SDK returns after FB.login(), independently re-verifies it
// against Facebook's own Graph API (see FacebookOAuthService.VerifyAccessToken —
// this confirms the token is genuinely valid AND was issued to this exact
// app, not just any Facebook app), then finds-or-creates the matching
// customer the same way GoogleLogin does.
func (ctrl *CustomerController) FacebookLogin(c *gin.Context) {
	var in facebookLoginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	fbUser, err := ctrl.Facebook.VerifyAccessToken(in.AccessToken)
	if err != nil {
		if errors.Is(err, services.ErrFacebookNotConfigured) {
			utils.InternalError(c, "facebook sign-in is not available right now")
			return
		}
		utils.Unauthorized(c, "មិនអាចផ្ទៀងផ្ទាត់គណនី Facebook បានទេ")
		return
	}

	customer, err := ctrl.Service.LoginOrRegisterWithFacebook(fbUser.ID, fbUser.Email, fbUser.Name)
	if err != nil {
		if errors.Is(err, services.ErrCustomerInactive) {
			utils.Forbidden(c, "គណនីនេះត្រូវបានផ្អាក សូមទាក់ទងមកយើង")
			return
		}
		utils.InternalError(c, "failed to sign in with facebook")
		return
	}

	token, err := utils.SignCustomerToken(customer.ID, customerTokenIdentifier(customer))
	if err != nil {
		utils.InternalError(c, "failed to sign in")
		return
	}
	utils.OK(c, gin.H{"token": token, "customer": customerJSON(customer)})
}

// GET /api/customer/me — requires customer auth
func (ctrl *CustomerController) Me(c *gin.Context) {
	customer, err := ctrl.Service.GetByID(middlewares.CurrentCustomerID(c))
	if err != nil {
		utils.NotFound(c, "customer not found")
		return
	}
	utils.OK(c, customerJSON(customer))
}

type updateProfileInput struct {
	Name  string `json:"name" validate:"required"`
	Phone string `json:"phone" validate:"omitempty"`
	Email string `json:"email" validate:"omitempty,email"`
}

// PUT /api/customer/me — requires customer auth — update full profile info
func (ctrl *CustomerController) UpdateProfile(c *gin.Context) {
	var in updateProfileInput

	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}

	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	customerID := middlewares.CurrentCustomerID(c)

	customer, err := ctrl.Service.UpdateProfile(
		customerID,
		in.Name,
		in.Phone,
		in.Email,
	)

	if err != nil {
		if errors.Is(err, services.ErrPhoneAlreadyRegistered) {
			utils.FailWithErrors(c, map[string]string{
				"phone": "លេខទូរស័ព្ទនេះបានចុះឈ្មោះរួចហើយ",
			})
			return
		}

		if errors.Is(err, services.ErrEmailAlreadyRegistered) {
			utils.FailWithErrors(c, map[string]string{
				"email": "អ៊ីមែលនេះបានចុះឈ្មោះរួចហើយ",
			})
			return
		}

		utils.InternalError(c, "failed to update profile")
		return
	}

	utils.OK(c, customerJSON(customer))
}

type customerChangePasswordInput struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required,min=6"`
}

// POST /api/customer/me/change-password — requires customer auth
func (ctrl *CustomerController) ChangePassword(c *gin.Context) {
	var in customerChangePasswordInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	if err := ctrl.Service.ChangePassword(middlewares.CurrentCustomerID(c), in.CurrentPassword, in.NewPassword); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.OK(c, gin.H{"message": "password updated"})
}
