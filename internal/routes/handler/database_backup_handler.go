package handler

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	appconfig "github.com/Bebii-Digital/bebii-blog/config"
	"github.com/gin-gonic/gin"
)

const maxSQLImportBytes = 128 << 20 // 128 MB

type DatabaseBackupHandler struct {
	cfg appconfig.Config
}

func NewDatabaseBackupHandler(cfg appconfig.Config) *DatabaseBackupHandler {
	return &DatabaseBackupHandler{cfg: cfg}
}

func (h *DatabaseBackupHandler) ExportSQL(c *gin.Context) {
	driver := strings.ToLower(strings.TrimSpace(h.cfg.DBDriver))
	var (
		out []byte
		err error
	)
	switch driver {
	case "mysql":
		out, err = h.exportMySQL()
	case "postgres", "postgresql":
		out, err = h.exportPostgres()
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported DB_DRIVER: " + driver})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "export failed",
			"detail": err.Error(),
		})
		return
	}
	filename := fmt.Sprintf("%s-%s.sql", safeFilePart(h.cfg.DBName), time.Now().UTC().Format("20060102-150405"))
	c.Header("Content-Type", "application/sql; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/sql; charset=utf-8", out)
}

func (h *DatabaseBackupHandler) ImportSQL(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	if fh.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty file"})
		return
	}
	if fh.Size > maxSQLImportBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large (max 128MB)"})
		return
	}
	f, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to open upload"})
		return
	}
	defer f.Close()
	sqlBytes, err := io.ReadAll(io.LimitReader(f, maxSQLImportBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read upload"})
		return
	}
	if len(sqlBytes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty file"})
		return
	}
	if len(sqlBytes) > maxSQLImportBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large (max 128MB)"})
		return
	}
	name := strings.ToLower(strings.TrimSpace(fh.Filename))
	if strings.HasSuffix(name, ".gz") {
		gr, err := gzip.NewReader(bytes.NewReader(sqlBytes))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid gzip file", "detail": err.Error()})
			return
		}
		defer gr.Close()
		decoded, err := io.ReadAll(io.LimitReader(gr, maxSQLImportBytes+1))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read gzip content", "detail": err.Error()})
			return
		}
		if len(decoded) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "empty SQL content inside gzip"})
			return
		}
		if len(decoded) > maxSQLImportBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unzipped SQL too large (max 128MB)"})
			return
		}
		sqlBytes = decoded
	}

	driver := strings.ToLower(strings.TrimSpace(h.cfg.DBDriver))
	switch driver {
	case "mysql":
		err = h.importMySQL(sqlBytes)
	case "postgres", "postgresql":
		err = h.importPostgres(sqlBytes)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported DB_DRIVER: " + driver})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "import failed",
			"detail": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "database restored successfully"})
}

func (h *DatabaseBackupHandler) exportMySQL() ([]byte, error) {
	var lastErr error
	for _, bin := range []string{"mysqldump", "mariadb-dump"} {
		if _, err := exec.LookPath(bin); err != nil {
			lastErr = err
			continue
		}
		args := []string{
			"--host=" + strings.TrimSpace(h.cfg.DBHost),
			"--port=" + strings.TrimSpace(h.cfg.DBPort),
			"--user=" + strings.TrimSpace(h.cfg.DBUser),
			"--password=" + h.cfg.DBPassword,
			"--single-transaction",
			"--skip-lock-tables",
			"--default-character-set=utf8mb4",
			strings.TrimSpace(h.cfg.DBName),
		}
		cmd := exec.Command(bin, args...)
		var out bytes.Buffer
		var errBuf bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(errBuf.String())
			if msg == "" {
				msg = err.Error()
			}
			lastErr = fmt.Errorf("%s failed: %s", bin, msg)
			continue
		}
		return out.Bytes(), nil
	}
	if lastErr == nil {
		lastErr = errors.New("mysqldump command not found")
	}
	return nil, fmt.Errorf("mysql export error: %w. install mysql client tools (mysqldump/mariadb-dump) and ensure PATH is set", lastErr)
}

func (h *DatabaseBackupHandler) importMySQL(sqlBytes []byte) error {
	var lastErr error
	for _, bin := range []string{"mysql", "mariadb"} {
		if _, err := exec.LookPath(bin); err != nil {
			lastErr = err
			continue
		}
		args := []string{
			"--host=" + strings.TrimSpace(h.cfg.DBHost),
			"--port=" + strings.TrimSpace(h.cfg.DBPort),
			"--user=" + strings.TrimSpace(h.cfg.DBUser),
			"--password=" + h.cfg.DBPassword,
			strings.TrimSpace(h.cfg.DBName),
		}
		cmd := exec.Command(bin, args...)
		cmd.Stdin = bytes.NewReader(sqlBytes)
		var errBuf bytes.Buffer
		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(errBuf.String())
			outMsg := strings.TrimSpace(outBuf.String())
			if msg == "" && outMsg != "" {
				msg = outMsg
			}
			if msg == "" {
				msg = err.Error()
			}
			lastErr = fmt.Errorf("%s restore failed: %s", bin, msg)
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("mysql command not found")
	}
	return fmt.Errorf("mysql import error: %w. install mysql client tools (mysql/mariadb) and ensure PATH is set", lastErr)
}

func (h *DatabaseBackupHandler) exportPostgres() ([]byte, error) {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		return nil, fmt.Errorf("pg_dump not found in PATH: %w", err)
	}
	args := []string{
		"-h", strings.TrimSpace(h.cfg.DBHost),
		"-p", strings.TrimSpace(h.cfg.DBPort),
		"-U", strings.TrimSpace(h.cfg.DBUser),
		"-d", strings.TrimSpace(h.cfg.DBName),
		"--clean",
		"--if-exists",
		"--no-owner",
		"--no-privileges",
	}
	cmd := exec.Command("pg_dump", args...)
	cmd.Env = append(cmd.Environ(), "PGPASSWORD="+h.cfg.DBPassword)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("pg_dump failed: %s", msg)
	}
	return out.Bytes(), nil
}

func (h *DatabaseBackupHandler) importPostgres(sqlBytes []byte) error {
	if _, err := exec.LookPath("psql"); err != nil {
		return fmt.Errorf("psql not found in PATH: %w", err)
	}
	args := []string{
		"-h", strings.TrimSpace(h.cfg.DBHost),
		"-p", strings.TrimSpace(h.cfg.DBPort),
		"-U", strings.TrimSpace(h.cfg.DBUser),
		"-d", strings.TrimSpace(h.cfg.DBName),
		"-v",
	}
	cmd := exec.Command("psql", args...)
	cmd.Env = append(cmd.Environ(), "PGPASSWORD="+h.cfg.DBPassword)
	cmd.Stdin = bytes.NewReader(sqlBytes)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("psql restore failed: %s", msg)
	}
	return nil
}

func safeFilePart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "database"
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "database"
	}
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}
