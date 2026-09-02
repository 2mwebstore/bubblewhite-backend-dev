package controllers

import (
	authdto "bubblewhite-backend/dto/auth"
	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	Service *services.AuthService
}

func NewAuthController(s *services.AuthService) *AuthController {
	return &AuthController{Service: s}
}

// POST /api/auth/login
func (ctrl *AuthController) Login(c *gin.Context) {
	var req authdto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(req); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	res, err := ctrl.Service.Login(req)
	if err != nil {
		utils.Unauthorized(c, err.Error())
		return
	}

	utils.OK(c, res)
}

// POST /api/admin/me/change-password (requires auth)
func (ctrl *AuthController) ChangePassword(c *gin.Context) {
	var req authdto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(req); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	userID := middlewares.CurrentUserID(c)
	if err := ctrl.Service.ChangePassword(userID, req.CurrentPassword, req.NewPassword); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.OK(c, gin.H{"message": "password updated"})
}

// GET /api/admin/me (requires auth) — returns the current user with their
// role AND that role's permission list, so the frontend can gate the admin
// UI correctly regardless of which role is logged in.
func (ctrl *AuthController) Me(c *gin.Context) {
	userID := middlewares.CurrentUserID(c)
	user, err := ctrl.Service.Me(userID)
	if err != nil {
		utils.NotFound(c, "user not found")
		return
	}

	utils.OK(c, gin.H{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
		"role": gin.H{
			"id":          user.Role.ID,
			"name":        user.Role.Name,
			"slug":        user.Role.Slug,
			"permissions": user.Role.Permissions.Data,
		},
	})
}
