package media

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const publicDir = "public"

// ResolvePublicFile maps a resizable /static/... URL to a file under public/.
func ResolvePublicFile(src string) (absPath string, relURL string, err error) {
	rel := NormalizeAssetPath(src)
	if rel == "" || !IsResizableAsset(rel) {
		return "", "", fmt.Errorf("asset not resizable")
	}
	clean, err := cleanSiteStaticPath(rel)
	if err != nil {
		return "", "", err
	}
	relPath := strings.TrimPrefix(clean, "/")
	abs := filepath.Join(publicDir, filepath.FromSlash(relPath))
	abs, err = filepath.Abs(abs)
	if err != nil {
		return "", "", err
	}
	pubAbs, err := filepath.Abs(publicDir)
	if err != nil {
		return "", "", err
	}
	if !isUnderDir(abs, pubAbs) {
		return "", "", fmt.Errorf("path outside public")
	}
	return abs, clean, nil
}

// cleanSiteStaticPath normalizes a URL-style /static/... path (forward slashes, all platforms).
func cleanSiteStaticPath(rel string) (string, error) {
	if !strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("invalid path")
	}
	clean := path.Clean(rel)
	if clean != rel {
		return "", fmt.Errorf("invalid path")
	}
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("invalid path")
	}
	if !IsResizableAsset(clean) {
		return "", fmt.Errorf("asset not resizable")
	}
	return clean, nil
}

func isUnderDir(abs, dir string) bool {
	abs = filepath.Clean(abs)
	dir = filepath.Clean(dir)
	if abs == dir {
		return true
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(abs, dir+sep)
}
