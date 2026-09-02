package controllers

import (
	"strconv"

	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/models"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	Service *services.UserService
	Roles   *services.RoleService
}

func NewUserController(s *services.UserService, roles *services.RoleService) *UserController {
	return &UserController{Service: s, Roles: roles}
}

// canAssignRole reports whether the CALLER is allowed to hand out roleID.
// Only an existing Administrator can grant the Administrator role to anyone
// (including a brand-new user) — otherwise a lower-privileged staff member
// could just create a new admin account for themselves. Any non-admin role
// can be assigned freely by whoever already holds user.create/user.manage.
func (ctrl *UserController) canAssignRole(c *gin.Context, roleID uint) bool {
	role, err := ctrl.Roles.GetByID(roleID)
	if err != nil {
		return false
	}
	if role.Slug != services.AdminRoleSlug {
		return true
	}
	return middlewares.CurrentRoleSlug(c) == services.AdminRoleSlug
}

// GET /api/admin/users (requires user.view)
func (ctrl *UserController) List(c *gin.Context) {
	users, err := ctrl.Service.List()
	if err != nil {
		utils.InternalError(c, "failed to fetch users")
		return
	}
	utils.OK(c, users)
}

// GET /api/admin/users/:id (requires user.view)
func (ctrl *UserController) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid user id")
		return
	}
	user, err := ctrl.Service.GetByID(uint(id))
	if err != nil {
		utils.NotFound(c, "user not found")
		return
	}
	utils.OK(c, user)
}

type createUserInput struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	RoleID   uint   `json:"roleId" validate:"required"`
}

// POST /api/admin/users (requires user.create)
func (ctrl *UserController) Create(c *gin.Context) {
	var in createUserInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	if !ctrl.canAssignRole(c, in.RoleID) {
		utils.Forbidden(c, "only an Administrator can assign the Administrator role")
		return
	}

	user := models.User{Name: in.Name, Email: in.Email, RoleID: in.RoleID, IsActive: true}
	if err := ctrl.Service.Create(&user, in.Password); err != nil {
		utils.InternalError(c, "failed to create user (email may already be in use)")
		return
	}
	utils.Created(c, user)
}

type updateUserInput struct {
	Name     string `json:"name" validate:"required"`
	IsActive *bool  `json:"isActive"`
}

// PUT /api/admin/users/:id (requires user.update)
func (ctrl *UserController) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid user id")
		return
	}

	user, err := ctrl.Service.GetByID(uint(id))
	if err != nil {
		utils.NotFound(c, "user not found")
		return
	}

	var in updateUserInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}

	user.Name = in.Name
	if in.IsActive != nil {
		user.IsActive = *in.IsActive
	}

	if err := ctrl.Service.Update(user); err != nil {
		utils.InternalError(c, "failed to update user")
		return
	}
	utils.OK(c, user)
}

// DELETE /api/admin/users/:id (requires user.delete)
func (ctrl *UserController) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid user id")
		return
	}
	if err := ctrl.Service.Delete(uint(id)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.NoContent(c)
}

type assignRoleInput struct {
	RoleID uint `json:"roleId" validate:"required"`
}

// PATCH /api/admin/users/:id/role (requires user.manage)
// This is "assign a role to a user" — the mechanism for granting permissions
// to a specific staff member, since permissions live on the Role, not the User.
func (ctrl *UserController) AssignRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid user id")
		return
	}

	var in assignRoleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}

	if !ctrl.canAssignRole(c, in.RoleID) {
		utils.Forbidden(c, "only an Administrator can assign the Administrator role")
		return
	}

	if err := ctrl.Service.AssignRole(uint(id), in.RoleID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.OK(c, gin.H{"message": "role assigned"})
}

type resetPasswordInput struct {
	NewPassword string `json:"newPassword" validate:"required,min=6"`
}

// PATCH /api/admin/users/:id/password (requires user.update)
// Admin-initiated reset — sets a new password for ANOTHER user directly, no
// current password needed (unlike the self-service change-password flow at
// POST /api/admin/me/change-password).
func (ctrl *UserController) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid user id")
		return
	}

	var in resetPasswordInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	if err := ctrl.Service.ResetPassword(uint(id), in.NewPassword); err != nil {
		utils.InternalError(c, "failed to reset password")
		return
	}
	utils.OK(c, gin.H{"message": "password reset"})
}
