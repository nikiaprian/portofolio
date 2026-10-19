package scraper

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const scrapedImageDirRel = "public/static/uploads/posts"

var moduleRootOnce sync.Once
var moduleRoot string

func findModuleRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func moduleRootDir() string {
	moduleRootOnce.Do(func() {
		moduleRoot = findModuleRoot()
	})
	return moduleRoot
}

// scrapedPostUploadDir returns the filesystem directory for scraped/post web images.
// Override in tests with BEBII_SCRAPED_IMAGE_DIR. Otherwise resolves from module root
// (directory containing go.mod), not the process working directory.
func scrapedPostUploadDir() string {
	if dir := strings.TrimSpace(os.Getenv("BEBII_SCRAPED_IMAGE_DIR")); dir != "" {
		return dir
	}
	if root := moduleRootDir(); root != "" {
		return filepath.Join(root, filepath.FromSlash(scrapedImageDirRel))
	}
	return scrapedImageDirRel
}
