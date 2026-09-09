package controllers

import (
	"errors"

	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type AdminBackupController struct {
	Service *services.BackupService
	Audit   *services.AuditLogService
}

func NewAdminBackupController(s *services.BackupService, audit *services.AuditLogService) *AdminBackupController {
	return &AdminBackupController{Service: s, Audit: audit}
}

// POST /api/admin/backup/run (requires settings.update) — triggers an
// immediate, on-demand backup, alongside the automatic daily midnight run
// (see BackupService.StartScheduler). Runs synchronously: for a store
// this size the dump+Telegram upload comfortably finishes well within
// the server's own 35s write timeout, so there's no real benefit to the
// added complexity of a background-job-plus-polling design here — if the
// database ever grows enough that this stops being true, that's the
// point to revisit this as async.
func (ctrl *AdminBackupController) RunNow(c *gin.Context) {
	err := ctrl.Service.RunBackup()

	ip, ua := auditContext(c)
	if err != nil {
		ctrl.Audit.Log(services.LogEntry{
			ActorType: "admin", ActorID: middlewares.CurrentUserID(c), ActorName: middlewares.CurrentUserEmail(c),
			Action: "backup_failed", Resource: "backup", Description: "Manual backup failed: " + err.Error(),
			IPAddress: ip, UserAgent: ua,
		})

		if errors.Is(err, services.ErrBackupNotConfigured) {
			utils.FailWithErrors(c, map[string]string{"backupTelegramGroupId": "សូមកំណត់លេខសម្គាល់ក្រុម Telegram សិន"})
			return
		}
		if errors.Is(err, services.ErrTelegramBotNotConfigured) {
			utils.FailWithErrors(c, map[string]string{"backupTelegramBotToken": "សូមកំណត់ Telegram Bot Token (ក្នុងការកំណត់ ឬ TELEGRAM_BOT_TOKEN) សិន"})
			return
		}
		if errors.Is(err, services.ErrBackupAlreadyRunning) {
			utils.Forbidden(c, "ការបម្រុងទុកកំពុងដំណើរការរួចហើយ សូមរង់ចាំបន្តិច")
			return
		}
		utils.InternalError(c, "មិនអាចបម្រុងទុកបានទេ: "+err.Error())
		return
	}

	ctrl.Audit.Log(services.LogEntry{
		ActorType: "admin", ActorID: middlewares.CurrentUserID(c), ActorName: middlewares.CurrentUserEmail(c),
		Action: "backup", Resource: "backup", Description: "Triggered a manual database backup",
		IPAddress: ip, UserAgent: ua,
	})
	utils.OK(c, gin.H{"message": "ការបម្រុងទុកបានផ្ញើទៅ Telegram ដោយជោគជ័យ"})
}
