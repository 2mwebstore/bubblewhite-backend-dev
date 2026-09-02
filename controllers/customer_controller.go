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
	Service *services.CustomerService
}

func NewCustomerController(s *services.CustomerService) *CustomerController {
	return &CustomerController{Service: s}
}

// customerJSON builds a consistent response shape across register/login/me/
// update/admin-list — Email is a *string on the model (nil when not set),
// flattened here to an empty string so the frontend never has to
// special-case null.
func customerJSON(customer *models.Customer) gin.H {
	email := ""
	if customer.Email != nil {
		email = *customer.Email
	}
	return gin.H{
		"id":        customer.ID,
		"name":      customer.Name,
		"phone":     customer.Phone,
		"email":     email,
		"isActive":  customer.IsActive,
		"createdAt": customer.CreatedAt,
	}
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

	token, err := utils.SignCustomerToken(customer.ID, customer.Phone)
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
		utils.Unauthorized(c, "លេខទូរស័ព្ទ/អ៊ីមែល ឬពាក្យសម្ងាត់មិនត្រឹមត្រូវ")
		return
	}

	token, err := utils.SignCustomerToken(customer.ID, customer.Phone)
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
	Phone string `json:"phone" validate:"required"`
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
