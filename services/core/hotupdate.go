package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HotUpdateService 提供 uni-app wgt 资源热更新的版本查询与文件下载。
//
// 约定：
//   - wgt 包放在 cfg.HotUpdateDir，命名 <versionName>_<versionCode>.wgt，例如 8.1.1_211.wgt
//   - 同目录可选放 <同名>.json 描述文件，用于覆盖 note / min_app_code / force 等元信息
//   - 客户端携带自身 versionCode 请求，仅当存在更高 versionCode 且满足 min_app_code 才返回更新
type HotUpdateService struct {
	dir     string
	baseURL string
	logger  *slog.Logger

	mu       sync.RWMutex
	cached   []wgtPackage
	cachedAt time.Time
}

type wgtPackage struct {
	VersionName string `json:"version_name"`
	VersionCode int64  `json:"version_code"`
	FileName    string `json:"file_name"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	Note        string `json:"note"`
	// MinAppCode 为可安装该 wgt 的最低原生壳 versionCode，0 表示不限制。
	// wgt 只能替换 JS/CSS 资源，若新版依赖了新的原生模块，必须由壳升级承载。
	MinAppCode int64 `json:"min_app_code"`
	Force      bool  `json:"force"`
	ModTime    int64 `json:"mod_time"`
}

type wgtMeta struct {
	Note       string `json:"note"`
	MinAppCode int64  `json:"min_app_code"`
	Force      bool   `json:"force"`
}

func NewHotUpdateService(dir, baseURL string, logger *slog.Logger) *HotUpdateService {
	return &HotUpdateService{
		dir:     dir,
		baseURL: strings.TrimRight(baseURL, "/"),
		logger:  logger,
	}
}

// scan 读取 wgt 目录，结果缓存 30 秒，避免频繁计算 sha256。
func (h *HotUpdateService) scan() ([]wgtPackage, error) {
	h.mu.RLock()
	if time.Since(h.cachedAt) < 30*time.Second && h.cached != nil {
		defer h.mu.RUnlock()
		return h.cached, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	entries, err := os.ReadDir(h.dir)
	if err != nil {
		if os.IsNotExist(err) {
			h.cached, h.cachedAt = []wgtPackage{}, time.Now()
			return h.cached, nil
		}
		return nil, err
	}

	packages := make([]wgtPackage, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".wgt") {
			continue
		}
		versionName, versionCode, ok := parseWgtName(entry.Name())
		if !ok {
			h.logger.Warn("skip wgt with unexpected name", "file", entry.Name())
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		full := filepath.Join(h.dir, entry.Name())
		sum, err := fileSHA256(full)
		if err != nil {
			h.logger.Warn("hash wgt failed", "file", entry.Name(), "error", err)
			continue
		}
		pkg := wgtPackage{
			VersionName: versionName,
			VersionCode: versionCode,
			FileName:    entry.Name(),
			Size:        info.Size(),
			SHA256:      sum,
			ModTime:     info.ModTime().Unix(),
		}
		if meta, err := readWgtMeta(strings.TrimSuffix(full, filepath.Ext(full)) + ".json"); err == nil {
			pkg.Note = meta.Note
			pkg.MinAppCode = meta.MinAppCode
			pkg.Force = meta.Force
		}
		packages = append(packages, pkg)
	}

	sort.Slice(packages, func(i, j int) bool { return packages[i].VersionCode > packages[j].VersionCode })
	h.cached, h.cachedAt = packages, time.Now()
	return packages, nil
}

// Check 返回给定客户端版本可用的最新 wgt。
func (h *HotUpdateService) Check(ctx context.Context, currentCode int64, appCode int64) (map[string]any, error) {
	packages, err := h.scan()
	if err != nil {
		return nil, err
	}
	if appCode <= 0 {
		appCode = currentCode
	}

	result := map[string]any{
		"has_update":   "0",
		"current_code": strconv.FormatInt(currentCode, 10),
		"server_time":  strconv.FormatInt(time.Now().Unix(), 10),
	}

	for _, pkg := range packages {
		if pkg.VersionCode <= currentCode {
			continue
		}
		// 新资源要求更高的原生壳版本时跳过，提示走整包升级。
		if pkg.MinAppCode > 0 && appCode < pkg.MinAppCode {
			result["native_upgrade_required"] = "1"
			result["min_app_code"] = strconv.FormatInt(pkg.MinAppCode, 10)
			continue
		}
		result["has_update"] = "1"
		result["version_name"] = pkg.VersionName
		result["version_code"] = strconv.FormatInt(pkg.VersionCode, 10)
		result["size"] = strconv.FormatInt(pkg.Size, 10)
		result["sha256"] = pkg.SHA256
		result["note"] = pkg.Note
		result["force"] = map[bool]string{true: "1", false: "0"}[pkg.Force]
		result["wgt_url"] = h.baseURL + "/appapi/hotupdate/download?file=" + pkg.FileName
		break
	}
	return result, nil
}

// Download 输出 wgt 文件本体。
func (h *HotUpdateService) Download(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(strings.TrimSpace(r.URL.Query().Get("file")))
	if name == "" || name == "." || name == "/" || !strings.HasSuffix(strings.ToLower(name), ".wgt") {
		http.Error(w, "invalid file", http.StatusBadRequest)
		return
	}
	full := filepath.Join(h.dir, name)
	// 防目录穿越：解析后必须仍在 wgt 目录内。
	absDir, err := filepath.Abs(h.dir)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	absFile, err := filepath.Abs(full)
	if err != nil || !strings.HasPrefix(absFile, absDir+string(os.PathSeparator)) {
		http.Error(w, "invalid file", http.StatusBadRequest)
		return
	}
	file, err := os.Open(absFile)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	http.ServeContent(w, r, name, info.ModTime(), file)
}

func parseWgtName(name string) (string, int64, bool) {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	idx := strings.LastIndex(base, "_")
	if idx <= 0 || idx == len(base)-1 {
		return "", 0, false
	}
	code, err := strconv.ParseInt(base[idx+1:], 10, 64)
	if err != nil || code <= 0 {
		return "", 0, false
	}
	return base[:idx], code, true
}

func readWgtMeta(path string) (wgtMeta, error) {
	var meta wgtMeta
	raw, err := os.ReadFile(path)
	if err != nil {
		return meta, err
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return meta, err
	}
	return meta, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
