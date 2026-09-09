package services

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"bubblewhite-backend/config"
	"bubblewhite-backend/repositories"

	"gorm.io/gorm"
)

// BackupService produces a full SQL dump of the database and delivers it
// to Telegram — deliberately implemented in pure Go rather than shelling
// out to the mysqldump CLI. A typical Go app's deployment container
// (Railway's Nixpacks builder, a minimal Docker image, etc.) only
// installs the Go toolchain and this app's own dependencies — it does
// NOT include MySQL's client tools by default, so relying on an external
// mysqldump binary would very likely fail silently or loudly the first
// time this actually runs in production, not in any way visible during
// development. Querying the schema and data directly through the
// existing database/sql connection avoids that dependency entirely.
type BackupService struct {
	DB       *gorm.DB
	Settings *repositories.SettingsRepository
	// running guards against two backups ever executing at once — a full
	// table scan of every table twice in parallel (the scheduled 12PM
	// run landing at the exact moment an admin clicks "Backup now", or a
	// double-click on that same button) wastes resources for no benefit,
	// since the second run would just produce a near-duplicate of the
	// first. int32 + atomic rather than a bool + mutex: the only
	// operation needed is a single compare-and-swap to claim the run,
	// which atomic.CompareAndSwapInt32 does directly without a separate
	// lock/unlock pair to get right.
	running int32
}

func NewBackupService(db *gorm.DB, settings *repositories.SettingsRepository) *BackupService {
	return &BackupService{DB: db, Settings: settings}
}

var ErrBackupNotConfigured = errors.New("no Telegram group configured for backups — set one in Settings first")
var ErrBackupAlreadyRunning = errors.New("a backup is already in progress")
var ErrTelegramBotNotConfigured = errors.New("TELEGRAM_BOT_TOKEN is not set on the server")

// RunBackup generates the dump and sends it to whichever Telegram group is
// currently configured in Settings.BackupTelegramGroupID. Returns an error
// rather than only logging internally, so a caller that DOES have someone
// to report to — the manual "Backup now" admin endpoint — can surface a
// real, specific message instead of a silent no-op; the scheduled
// midnight caller (see StartScheduler) still just logs whatever comes
// back, since there's no one waiting on that call to report to.
//
// Wrapped in its own panic recovery (see the deferred recover below) so
// that ANY unexpected panic anywhere in this flow — a nil pointer, a
// slice index, anything not yet exercised — turns into a proper,
// descriptive error returned to the caller, rather than being caught
// by Gin's own top-level recovery middleware, which returns a bare 500
// with no message at all. If this backup is still failing after this
// change, checking Railway's own server logs for the "backup:" lines
// logged at each stage below is the most direct way to see exactly
// where it's failing — that's not something visible from here.
func (s *BackupService) RunBackup() (err error) {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return ErrBackupAlreadyRunning
	}
	defer atomic.StoreInt32(&s.running, 0)

	defer func() {
		if r := recover(); r != nil {
			log.Printf("backup: PANIC recovered: %v\n%s", r, debug.Stack())
			err = fmt.Errorf("internal error during backup (panic): %v", r)
		}
	}()

	log.Println("backup: starting")

	settings, loadErr := s.Settings.Get()
	if loadErr != nil {
		return fmt.Errorf("loading settings: %w", loadErr)
	}
	groupID := settings.BackupTelegramGroupID
	if groupID == "" {
		return ErrBackupNotConfigured
	}

	// Prefers the dedicated Settings-based token (BackupTelegramBotToken)
	// over the shared TELEGRAM_BOT_TOKEN env var — see that field's own
	// doc comment on models.Settings for exactly why a separate bot is
	// worth having. Falling back to the shared env var when the
	// dedicated one isn't set keeps this working for anyone who hasn't
	// configured the new field yet, rather than breaking backups that
	// were already working.
	botToken := settings.BackupTelegramBotToken
	if botToken == "" {
		botToken = config.Get().TelegramBotToken
	}
	if botToken == "" {
		return ErrTelegramBotNotConfigured
	}

	log.Println("backup: generating SQL dump")
	dump, genErr := s.generateDump()
	if genErr != nil {
		return fmt.Errorf("generating dump: %w", genErr)
	}
	log.Printf("backup: dump generated, %d bytes", len(dump))

	// Telegram bots can only send files up to 50MB via sendDocument —
	// caught explicitly here with a clear message, rather than letting
	// it fail as an opaque rejection from Telegram's API.
	const telegramMaxBytes = 50 * 1024 * 1024
	if len(dump) > telegramMaxBytes {
		return fmt.Errorf("dump is %d bytes, which exceeds Telegram's 50MB file size limit", len(dump))
	}

	loc, _ := time.LoadLocation("Asia/Phnom_Penh")
	filename := fmt.Sprintf("bubblewhite-backup-%s.sql", time.Now().In(loc).Format("2006-01-02_15-04-05"))
	caption := fmt.Sprintf("BubbleWhite database backup — %s (Phnom Penh time)", time.Now().In(loc).Format("2006-01-02 15:04"))

	log.Println("backup: sending to telegram")
	if sendErr := sendDocumentToTelegram(botToken, groupID, filename, caption, dump); sendErr != nil {
		return fmt.Errorf("sending to telegram: %w", sendErr)
	}
	log.Printf("backup: sent %s (%d bytes) to telegram group %s", filename, len(dump), groupID)
	return nil
}

// StartScheduler runs RunBackup once a day at 12:00 PM Phnom Penh time —
// a plain goroutine sleeping until the next occurrence, rather than
// pulling in a cron library for a single daily job. Call once at
// application startup.
func (s *BackupService) StartScheduler() {
	go func() {
		loc, err := time.LoadLocation("Asia/Phnom_Penh")
		if err != nil {
			log.Printf("backup: failed to load Asia/Phnom_Penh timezone, scheduler not started: %v", err)
			return
		}
		for {
			now := time.Now().In(loc)
			next := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc)
			if !now.Before(next) {
				next = next.Add(24 * time.Hour)
			}
			log.Printf("backup: next scheduled run at %s", next.Format(time.RFC1123))
			time.Sleep(next.Sub(now))
			if err := s.RunBackup(); err != nil {
				log.Printf("backup: scheduled run failed: %v", err)
			}
		}
	}()
}

// generateDump produces a complete, re-importable SQL dump: every table's
// CREATE TABLE statement followed by INSERT statements for its current
// rows, wrapped in the same DROP-then-CREATE pattern mysqldump itself
// uses so restoring this file onto an empty (or existing, to be
// overwritten) database just works without manual editing.
func (s *BackupService) generateDump() ([]byte, error) {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return nil, fmt.Errorf("getting underlying *sql.DB: %w", err)
	}

	tables, err := tableNames(sqlDB)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("-- BubbleWhite database backup\n")
	buf.WriteString(fmt.Sprintf("-- Generated at %s\n", time.Now().UTC().Format(time.RFC3339)))
	buf.WriteString("SET FOREIGN_KEY_CHECKS=0;\n\n")

	for _, table := range tables {
		createStmt, err := showCreateTable(sqlDB, table)
		if err != nil {
			return nil, fmt.Errorf("getting CREATE TABLE for %s: %w", table, err)
		}
		buf.WriteString(fmt.Sprintf("-- Table: %s\n", table))
		buf.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS `%s`;\n", table))
		buf.WriteString(createStmt)
		buf.WriteString(";\n\n")

		if err := writeInsertsForTable(&buf, sqlDB, table); err != nil {
			return nil, fmt.Errorf("dumping rows for %s: %w", table, err)
		}
		buf.WriteString("\n")
	}

	buf.WriteString("SET FOREIGN_KEY_CHECKS=1;\n")
	return buf.Bytes(), nil
}

func tableNames(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func showCreateTable(db *sql.DB, table string) (string, error) {
	row := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`", table))
	var tableName, createStmt string
	if err := row.Scan(&tableName, &createStmt); err != nil {
		return "", err
	}
	return createStmt, nil
}

// writeInsertsForTable streams INSERT statements directly into buf rather
// than building a slice of all rows in memory first — this table could
// hold years of orders/customers/audit logs by the time backups actually
// matter, and there's no reason to hold two full copies of that (the
// query result AND a rebuilt slice) in memory at once when the SQL text
// can just be written out row by row as it's read.
func writeInsertsForTable(buf *bytes.Buffer, db *sql.DB, table string) error {
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM `%s`", table))
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	if len(columns) == 0 {
		return nil
	}

	quotedColumns := make([]string, len(columns))
	for i, c := range columns {
		quotedColumns[i] = "`" + c + "`"
	}
	columnList := strings.Join(quotedColumns, ", ")

	values := make([]interface{}, len(columns))
	scanTargets := make([]interface{}, len(columns))
	for i := range values {
		scanTargets[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return err
		}

		formatted := make([]string, len(columns))
		for i, v := range values {
			formatted[i] = formatSQLValue(v)
		}
		buf.WriteString(fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s);\n", table, columnList, strings.Join(formatted, ", ")))
	}
	return rows.Err()
}

// formatSQLValue turns one scanned column value into its literal SQL
// text representation. The driver hands back a fairly small set of
// concrete Go types for MySQL columns (see the type switch below); the
// default case exists as a safety net for any type this hasn't been
// exercised against, not because it's expected to be reached.
func formatSQLValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case []byte:
		return "'" + escapeSQLString(string(val)) + "'"
	case string:
		return "'" + escapeSQLString(val) + "'"
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "1"
		}
		return "0"
	case time.Time:
		return "'" + val.Format("2006-01-02 15:04:05") + "'"
	default:
		return "'" + escapeSQLString(fmt.Sprintf("%v", val)) + "'"
	}
}

// escapeSQLString escapes the characters that would otherwise break out
// of a single-quoted SQL string literal — backslash first (so it doesn't
// double-escape the quotes/newlines escaped right after it), then the
// quote character itself, then the two whitespace characters MySQL
// specifically requires backslash-escapes for inside a string literal.
func escapeSQLString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}

// sendDocumentToTelegram uploads a file to a Telegram chat via the
// sendDocument API — multipart/form-data, since Telegram's file upload
// endpoints don't accept a JSON body the way sendMessage does.
func sendDocumentToTelegram(botToken, chatID, filename, caption string, data []byte) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("chat_id", chatID); err != nil {
		return err
	}
	if err := writer.WriteField("caption", caption); err != nil {
		return err
	}
	part, err := writer.CreateFormFile("document", filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", botToken)
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram sendDocument failed, status=%d body=%s", resp.StatusCode, string(respBody))
	}
	return nil
}
