package controllers

import (
	"errors"

	"bubblewhite-backend/config"
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
	Otp      *services.OtpService
	Audit    *services.AuditLogService
}

func NewCustomerController(s *services.CustomerService, google *services.GoogleOAuthService, facebook *services.FacebookOAuthService, otp *services.OtpService, audit *services.AuditLogService) *CustomerController {
	return &CustomerController{Service: s, Google: google, Facebook: facebook, Otp: otp, Audit: audit}
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

	// authProviders lists every sign-in method currently linked to this
	// account, not necessarily just the one used the very first time
	// they signed up — a customer can end up with more than one (e.g.
	// registered by phone, later signed in with Google using the same
	// email, which links rather than duplicates — see
	// CustomerService.LoginOrRegisterWithGoogle). For the large majority
	// of customers who only ever use one method, this is exactly "how
	// they registered"; for the rarer multi-provider case it's an
	// honest "these are the ways in", which is more useful to an admin
	// than guessing at which one came first.
	var authProviders []string
	if customer.GoogleID != nil {
		authProviders = append(authProviders, "google")
	}
	if customer.FacebookID != nil {
		authProviders = append(authProviders, "facebook")
	}
	if customer.PasswordHash != nil {
		authProviders = append(authProviders, "phone")
	}
	if authProviders == nil {
		authProviders = []string{}
	}

	return gin.H{
		"id":        customer.ID,
		"name":      customer.Name,
		"phone":     phone,
		"email":     email,
		"isActive":  customer.IsActive,
		"createdAt": customer.CreatedAt,
		// Lets the frontend's change-password form know whether to ask
		// for a "current password" at all — a customer who only ever
		// signed up via Google/Facebook has none, and requiring one
		// there would block them from ever setting a local password in
		// the first place (see CustomerService.ChangePassword, which
		// already handles this correctly server-side; this field is what
		// lets the UI match that instead of contradicting it).
		"hasPassword":   customer.PasswordHash != nil,
		"authProviders": authProviders,
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

type registerRequestInput struct {
	Name     string `json:"name" validate:"required"`
	Phone    string `json:"phone" validate:"required"`
	Email    string `json:"email" validate:"omitempty,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// POST /api/customer/register/request-otp — public. Replaces the old
// direct /customer/register endpoint: the full form is collected and
// validated HERE, up front — phone verification is the final step, not
// the first one. Nothing is actually created yet; this only checks the
// input is valid, checks phone/email aren't already taken, hashes the
// password, and sends the OTP. The account itself is only ever created
// inside VerifyOTP below, once the code is confirmed — see
// CustomerService.CreateFromVerifiedOtp and OtpRequest's own Pending*
// fields for how that data survives the wait between here and there.
func (ctrl *CustomerController) RegisterRequestOTP(c *gin.Context) {
	var in registerRequestInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	phone, err := utils.NormalizeCambodianPhone(in.Phone)
	if err != nil {
		utils.FailWithErrors(c, map[string]string{"phone": "លេខទូរស័ព្ទមិនត្រឹមត្រូវ"})
		return
	}

	// Checked now, before ever sending a code, so a phone/email that's
	// already taken fails immediately with a clear message rather than
	// wasting an OTP send (a real cost for the SMS fallback) on a
	// registration that was always going to fail. Re-checked again at
	// actual account-creation time too — see CreateFromVerifiedOtp's own
	// comment for why a check this far ahead of verification isn't
	// enough on its own.
	if err := ctrl.Service.CheckPhoneAndEmailAvailable(phone, in.Email); err != nil {
		if errors.Is(err, services.ErrPhoneAlreadyRegistered) {
			utils.FailWithErrors(c, map[string]string{"phone": "លេខទូរស័ព្ទនេះបានចុះឈ្មោះរួចហើយ"})
			return
		}
		if errors.Is(err, services.ErrEmailAlreadyRegistered) {
			utils.FailWithErrors(c, map[string]string{"email": "អ៊ីមែលនេះបានចុះឈ្មោះរួចហើយ"})
			return
		}
		utils.InternalError(c, "failed to check registration details")
		return
	}

	passwordHash, err := utils.HashPassword(in.Password)
	if err != nil {
		utils.InternalError(c, "failed to process password")
		return
	}

	result, err := ctrl.Otp.RequestRegistrationOTP(in.Name, phone, in.Email, passwordHash, config.Get().TelegramBotUsername)
	if err != nil {
		utils.InternalError(c, "failed to send verification code")
		return
	}
	utils.OK(c, gin.H{"channel": result.Channel, "telegramLinkUrl": result.TelegramLinkURL})
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
		ip, ua := auditContext(c)
		ctrl.Audit.Log(services.LogEntry{
			ActorType: "customer", Action: "login_failed", Resource: "auth",
			Description: "Failed login attempt for " + in.Identifier,
			IPAddress:   ip, UserAgent: ua,
		})
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

	ip, ua := auditContext(c)
	ctrl.Audit.Log(services.LogEntry{
		ActorType: "customer", ActorID: customer.ID, ActorName: customer.Name,
		Action: "login", Resource: "auth", Description: "Logged in with password",
		IPAddress: ip, UserAgent: ua,
	})
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
	ip, ua := auditContext(c)
	ctrl.Audit.Log(services.LogEntry{
		ActorType: "customer", ActorID: customer.ID, ActorName: customer.Name,
		Action: "login", Resource: "auth", Description: "Logged in with Google",
		IPAddress: ip, UserAgent: ua,
	})
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
	ip, ua := auditContext(c)
	ctrl.Audit.Log(services.LogEntry{
		ActorType: "customer", ActorID: customer.ID, ActorName: customer.Name,
		Action: "login", Resource: "auth", Description: "Logged in with Facebook",
		IPAddress: ip, UserAgent: ua,
	})
	utils.OK(c, gin.H{"token": token, "customer": customerJSON(customer)})
}

type otpRequestInput struct {
	Phone string `json:"phone" validate:"required"`
}

// POST /api/customer/otp/request — public. Tries the free Telegram
// channel first (either sending directly if this phone already has a
// linked chat, or returning a deep-link URL for the customer to tap
// through) — see OtpService.RequestOTP for the full channel-selection
// logic. Never falls back to paid SMS automatically; that's the
// customer's own explicit choice via RequestOTPBySMS below.
func (ctrl *CustomerController) RequestOTP(c *gin.Context) {
	var in otpRequestInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	phone, err := utils.NormalizeCambodianPhone(in.Phone)
	if err != nil {
		utils.FailWithErrors(c, map[string]string{"phone": "លេខទូរស័ព្ទមិនត្រឹមត្រូវ"})
		return
	}

	result, err := ctrl.Otp.RequestOTP(phone, config.Get().TelegramBotUsername)
	if err != nil {
		utils.InternalError(c, "failed to send verification code")
		return
	}
	utils.OK(c, gin.H{"channel": result.Channel, "telegramLinkUrl": result.TelegramLinkURL})
}

// POST /api/customer/otp/request-sms — public. The customer's own
// explicit fallback choice (no Telegram installed, or just a preference)
// — deliberately a separate, deliberate action rather than an automatic
// fallback from RequestOTP, since every call here costs real money
// through Plasgate.
func (ctrl *CustomerController) RequestOTPBySMS(c *gin.Context) {
	var in otpRequestInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	phone, err := utils.NormalizeCambodianPhone(in.Phone)
	if err != nil {
		utils.FailWithErrors(c, map[string]string{"phone": "លេខទូរស័ព្ទមិនត្រឹមត្រូវ"})
		return
	}

	if err := ctrl.Otp.RequestSMSFallback(phone); err != nil {
		utils.InternalError(c, "failed to send verification code")
		return
	}
	utils.OK(c, gin.H{"channel": "sms"})
}

type otpVerifyInput struct {
	Phone string `json:"phone" validate:"required"`
	Code  string `json:"code" validate:"required"`
}

// POST /api/customer/otp/verify — public. Checks the submitted code, then
// branches on whether this OtpRequest carries pending registration data
// (see OtpRequest's own Pending* fields):
//   - pending registration data present: this verification IS the final
//     step of registration — creates the account right now (see
//     CustomerService.CreateFromVerifiedOtp) and logs them in.
//   - no pending data: a plain login-via-OTP attempt. Existing account —
//     logs them in directly, same {token, customer} shape as every other
//     login endpoint. No account — tells the frontend to send the
//     customer to the registration form instead (there's no partial data
//     to complete here, unlike the registration-in-progress case above).
func (ctrl *CustomerController) VerifyOTP(c *gin.Context) {
	var in otpVerifyInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	phone, err := utils.NormalizeCambodianPhone(in.Phone)
	if err != nil {
		utils.FailWithErrors(c, map[string]string{"phone": "លេខទូរស័ព្ទមិនត្រឹមត្រូវ"})
		return
	}

	otp, err := ctrl.Otp.VerifyOTP(phone, in.Code)
	if err != nil {
		if errors.Is(err, services.ErrOtpTooManyAttempts) {
			utils.Forbidden(c, "ព្យាយាមខុសច្រើនដងពេក សូមស្នើសុំលេខកូដថ្មី")
			return
		}
		if errors.Is(err, services.ErrOtpExpiredOrNotFound) {
			utils.BadRequest(c, "លេខកូដបានផុតកំណត់ ឬមិនទាន់ស្នើសុំ សូមព្យាយាមម្តងទៀត")
			return
		}
		utils.BadRequest(c, "លេខកូដមិនត្រឹមត្រូវ")
		return
	}

	var customer *models.Customer
	var wasRegistration bool
	if otp.PendingName != nil && otp.PendingPasswordHash != nil {
		wasRegistration = true
		// This verification completes a registration — the account is
		// created RIGHT NOW, from the data collected back when the form
		// was first submitted (see RegisterRequestOTP), not from a
		// second request the customer has to make themselves.
		email := ""
		if otp.PendingEmail != nil {
			email = *otp.PendingEmail
		}
		customer, err = ctrl.Service.CreateFromVerifiedOtp(*otp.PendingName, phone, email, *otp.PendingPasswordHash)
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
	} else {
		// A plain login-via-OTP attempt — no pending registration data
		// on this request, so this phone either already has an account
		// or the customer needs to go fill out the registration form
		// instead (there's no partial data to fall back on here).
		customer, err = ctrl.Service.LoginWithVerifiedPhone(phone)
		if err != nil {
			if errors.Is(err, services.ErrNoAccountForPhone) {
				utils.OK(c, gin.H{"needsRegistration": true})
				return
			}
			if errors.Is(err, services.ErrCustomerInactive) {
				utils.Forbidden(c, "គណនីនេះត្រូវបានផ្អាក សូមទាក់ទងមកយើង")
				return
			}
			utils.InternalError(c, "failed to sign in")
			return
		}
	}

	token, err := utils.SignCustomerToken(customer.ID, customerTokenIdentifier(customer))
	if err != nil {
		utils.InternalError(c, "failed to sign in")
		return
	}

	ip, ua := auditContext(c)
	if wasRegistration {
		ctrl.Audit.Log(services.LogEntry{
			ActorType: "customer", ActorID: customer.ID, ActorName: customer.Name,
			Action: "register", Resource: "auth", Description: "Registered via phone OTP",
			IPAddress: ip, UserAgent: ua,
		})
	} else {
		ctrl.Audit.Log(services.LogEntry{
			ActorType: "customer", ActorID: customer.ID, ActorName: customer.Name,
			Action: "login", Resource: "auth", Description: "Logged in via phone OTP",
			IPAddress: ip, UserAgent: ua,
		})
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

	// Normalized when provided (see utils.NormalizeCambodianPhone's own
	// comment for why this matters for OTP login specifically) — but
	// Phone is optional here (a Google/Facebook customer may not have
	// one yet), so an empty value is passed through unchanged rather
	// than rejected.
	phone := in.Phone
	if phone != "" {
		normalized, err := utils.NormalizeCambodianPhone(phone)
		if err != nil {
			utils.FailWithErrors(c, map[string]string{"phone": "លេខទូរស័ព្ទមិនត្រឹមត្រូវ"})
			return
		}
		phone = normalized
	}

	customer, err := ctrl.Service.UpdateProfile(
		customerID,
		in.Name,
		phone,
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

	ip, ua := auditContext(c)
	ctrl.Audit.Log(services.LogEntry{
		ActorType: "customer", ActorID: customer.ID, ActorName: customer.Name,
		Action: "update", Resource: "customer_profile", Description: "Updated their profile",
		IPAddress: ip, UserAgent: ua,
	})
	utils.OK(c, customerJSON(customer))
}

type customerChangePasswordInput struct {
	// omitempty, not required — a customer who only ever signed up via
	// Google/Facebook has no existing password to provide (see
	// CustomerService.ChangePassword, which correctly skips verification
	// when PasswordHash is nil to let them set one for the first time).
	// Marking this required would block exactly that case at validation,
	// before the service's own handling of it is ever reached.
	CurrentPassword string `json:"currentPassword" validate:"omitempty"`
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

	customerID := middlewares.CurrentCustomerID(c)
	if err := ctrl.Service.ChangePassword(customerID, in.CurrentPassword, in.NewPassword); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	ip, ua := auditContext(c)
	actorName := ""
	if customer, err := ctrl.Service.GetByID(customerID); err == nil {
		actorName = customer.Name
	}
	ctrl.Audit.Log(services.LogEntry{
		ActorType: "customer", ActorID: customerID, ActorName: actorName,
		Action: "change_password", Resource: "customer_profile", Description: "Changed their password",
		IPAddress: ip, UserAgent: ua,
	})
	utils.OK(c, gin.H{"message": "password updated"})
}
