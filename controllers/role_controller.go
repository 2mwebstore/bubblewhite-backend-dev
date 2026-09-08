package controllers

import (
	"fmt"
	"strconv"
	"strings"

	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/models"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RoleController struct {
	Service *services.RoleService
	DB      *gorm.DB
	Audit   *services.AuditLogService
}

func NewRoleController(s *services.RoleService, db *gorm.DB, audit *services.AuditLogService) *RoleController {
	return &RoleController{Service: s, DB: db, Audit: audit}
}

// GET /api/admin/roles (requires role.view)
func (ctrl *RoleController) List(c *gin.Context) {
	roles, err := ctrl.Service.List()
	if err != nil {
		utils.InternalError(c, "failed to fetch roles")
		return
	}
	utils.OK(c, roles)
}

// GET /api/admin/permissions (requires role.view)
// Returns the full fixed permission catalog, so the admin UI can render a
// checklist of every grantable permission when creating/editing a role.
func (ctrl *RoleController) ListPermissions(c *gin.Context) {
	var permissions []models.Permission
	if err := ctrl.DB.Order("`group`, slug").Find(&permissions).Error; err != nil {
		utils.InternalError(c, "failed to fetch permissions")
		return
	}
	utils.OK(c, permissions)
}

type roleInput struct {
	Name        string   `json:"name" validate:"required"`
	Slug        string   `json:"slug" validate:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// disallowedPermissions checks the requested permission slugs against the
// CALLER's own role — nobody can create or edit a role with more power than
// they themselves have. An Administrator holds every permission explicitly
// (see seed/seed.go's seedRoles, which keeps that list synced to the full
// catalog on every boot — no "*" wildcard), so this check still passes for
// them as long as their own list is up to date. Without this check at all,
// an Editor could create a brand-new role with
// full admin permissions and hand it to themselves via user management.
// Returns the slugs the caller isn't allowed to grant; empty means OK.
func (ctrl *RoleController) disallowedPermissions(c *gin.Context, requested []string) ([]string, error) {
	callerRole, err := ctrl.Service.GetBySlug(middlewares.CurrentRoleSlug(c))
	if err != nil {
		return nil, err
	}
	if callerRole.HasPermission("*") {
		return nil, nil
	}
	owned := make(map[string]bool, len(callerRole.Permissions.Data))
	for _, p := range callerRole.Permissions.Data {
		owned[p] = true
	}
	var disallowed []string
	for _, p := range requested {
		if !owned[p] {
			disallowed = append(disallowed, p)
		}
	}
	return disallowed, nil
}

// POST /api/admin/roles (requires role.create)
func (ctrl *RoleController) Create(c *gin.Context) {
	var in roleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	disallowed, err := ctrl.disallowedPermissions(c, in.Permissions)
	if err != nil {
		utils.InternalError(c, "failed to verify permissions")
		return
	}
	if len(disallowed) > 0 {
		utils.Forbidden(c, fmt.Sprintf("you can't grant permissions you don't have yourself: %s", strings.Join(disallowed, ", ")))
		return
	}

	role := models.Role{
		Name:        in.Name,
		Slug:        in.Slug,
		Description: in.Description,
		Permissions: models.JSONColumn[[]string]{Data: in.Permissions},
	}
	if err := ctrl.Service.Create(&role); err != nil {
		utils.InternalError(c, "failed to create role")
		return
	}
	ip, ua := auditContext(c)
	ctrl.Audit.Log(services.LogEntry{
		ActorType: "admin", ActorID: middlewares.CurrentUserID(c), ActorName: middlewares.CurrentUserEmail(c),
		Action: "create", Resource: "role", ResourceID: strconv.FormatUint(uint64(role.ID), 10), ResourceLabel: role.Name,
		Description: "Created role", IPAddress: ip, UserAgent: ua,
	})
	utils.Created(c, role)
}

// PUT /api/admin/roles/:id (requires role.update)
// This is where an admin edits which permissions a role grants.
func (ctrl *RoleController) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid role id")
		return
	}

	role, err := ctrl.Service.GetByID(uint(id))
	if err != nil {
		utils.NotFound(c, "role not found")
		return
	}

	var in roleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}

	disallowed, err := ctrl.disallowedPermissions(c, in.Permissions)
	if err != nil {
		utils.InternalError(c, "failed to verify permissions")
		return
	}
	if len(disallowed) > 0 {
		utils.Forbidden(c, fmt.Sprintf("you can't grant permissions you don't have yourself: %s", strings.Join(disallowed, ", ")))
		return
	}

	role.Name = in.Name
	role.Description = in.Description
	role.Permissions = models.JSONColumn[[]string]{Data: in.Permissions}
	// Slug intentionally not overwritten for system roles — see service layer.

	if err := ctrl.Service.Update(role); err != nil {
		utils.InternalError(c, "failed to update role")
		return
	}
	ip, ua := auditContext(c)
	ctrl.Audit.Log(services.LogEntry{
		ActorType: "admin", ActorID: middlewares.CurrentUserID(c), ActorName: middlewares.CurrentUserEmail(c),
		Action: "update", Resource: "role", ResourceID: strconv.FormatUint(uint64(role.ID), 10), ResourceLabel: role.Name,
		Description: "Updated role permissions", IPAddress: ip, UserAgent: ua,
	})
	utils.OK(c, role)
}

// DELETE /api/admin/roles/:id (requires role.delete)
func (ctrl *RoleController) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid role id")
		return
	}
	label := c.Param("id")
	if role, err := ctrl.Service.GetByID(uint(id)); err == nil {
		label = role.Name
	}
	if err := ctrl.Service.Delete(uint(id)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	ip, ua := auditContext(c)
	ctrl.Audit.Log(services.LogEntry{
		ActorType: "admin", ActorID: middlewares.CurrentUserID(c), ActorName: middlewares.CurrentUserEmail(c),
		Action: "delete", Resource: "role", ResourceID: c.Param("id"), ResourceLabel: label,
		Description: "Deleted role", IPAddress: ip, UserAgent: ua,
	})
	utils.NoContent(c)
}
