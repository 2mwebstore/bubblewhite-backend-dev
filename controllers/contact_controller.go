package controllers

import (
	"strconv"

	"bubblewhite-backend/models"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type ContactController struct {
	Service *services.ContactService
}

func NewContactController(s *services.ContactService) *ContactController {
	return &ContactController{Service: s}
}

type contactInput struct {
	Name    string `json:"name" validate:"required"`
	Email   string `json:"email" validate:"required,email"`
	Subject string `json:"subject"`
	Message string `json:"message" validate:"required"`
}

// POST /api/contact — public, the storefront's Contact page form posts here.
func (ctrl *ContactController) Submit(c *gin.Context) {
	var in contactInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	msg := models.ContactMessage{
		Name:    in.Name,
		Email:   in.Email,
		Subject: in.Subject,
		Message: in.Message,
	}
	if err := ctrl.Service.Submit(&msg); err != nil {
		utils.InternalError(c, "failed to save message")
		return
	}

	utils.Created(c, gin.H{"message": "ទទួលបានសាររបស់អ្នកហើយ សូមអរគុណ!"})
}

// GET /api/admin/contacts (requires contact.view) — admin inbox listing.
func (ctrl *ContactController) List(c *gin.Context) {
	messages, err := ctrl.Service.List()
	if err != nil {
		utils.InternalError(c, "failed to fetch messages")
		return
	}
	utils.OK(c, messages)
}

// PATCH /api/admin/contacts/:id/read (requires contact.manage)
func (ctrl *ContactController) MarkRead(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	if err := ctrl.Service.MarkRead(uint(id)); err != nil {
		utils.InternalError(c, "failed to update message")
		return
	}
	utils.OK(c, gin.H{"message": "marked as read"})
}

// DELETE /api/admin/contacts/:id (requires contact.manage)
func (ctrl *ContactController) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid id")
		return
	}
	if err := ctrl.Service.Delete(uint(id)); err != nil {
		utils.InternalError(c, "failed to delete message")
		return
	}
	utils.NoContent(c)
}
