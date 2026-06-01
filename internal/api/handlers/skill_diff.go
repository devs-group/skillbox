package handlers

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/skill"
	"github.com/devs-group/skillbox/internal/store"
)

// diffLine is one line in a file diff: op is " " (context), "+" (added), "-" (removed).
type diffLine struct {
	Op   string `json:"op"`
	Text string `json:"text"`
}

// fileDiff describes how one file changed between two versions.
type fileDiff struct {
	Path   string     `json:"path"`
	Status string     `json:"status"` // added, removed, modified, unchanged
	Lines  []diffLine `json:"lines,omitempty"`
}

// SkillDiff handles GET /v1/skills/:name/diff — per-file line diff between two versions.
func SkillDiff(reg *registry.Registry, s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)
		name := c.Param("name")
		from := c.Query("from")
		to := c.Query("to")

		if err := skill.ValidateName(name); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if to == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "to version is required")
			return
		}
		if err := skill.ValidateVersion(to); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		// Default the base to the active version when not given.
		if from == "" {
			if active, err := s.ResolveActiveVersion(c.Request.Context(), tenantID, name); err == nil && active != to {
				from = active
			}
		}
		if from != "" {
			if err := skill.ValidateVersion(from); err != nil {
				response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
				return
			}
		}

		newFiles, err := loadVersionFiles(c, reg, tenantID, name, to)
		if err != nil {
			response.RespondError(c, http.StatusNotFound, "not_found", "version not found: "+name+"@"+to)
			return
		}
		oldFiles := map[string]string{}
		if from != "" {
			if f, err := loadVersionFiles(c, reg, tenantID, name, from); err == nil {
				oldFiles = f
			}
		}

		paths := map[string]struct{}{}
		for p := range oldFiles {
			paths[p] = struct{}{}
		}
		for p := range newFiles {
			paths[p] = struct{}{}
		}

		diffs := make([]fileDiff, 0, len(paths))
		for p := range paths {
			oldC, hadOld := oldFiles[p]
			newC, hadNew := newFiles[p]
			switch {
			case hadOld && !hadNew:
				diffs = append(diffs, fileDiff{Path: p, Status: "removed", Lines: lineDiff(oldC, "")})
			case !hadOld && hadNew:
				diffs = append(diffs, fileDiff{Path: p, Status: "added", Lines: lineDiff("", newC)})
			case oldC == newC:
				diffs = append(diffs, fileDiff{Path: p, Status: "unchanged"})
			default:
				diffs = append(diffs, fileDiff{Path: p, Status: "modified", Lines: lineDiff(oldC, newC)})
			}
		}

		c.JSON(http.StatusOK, gin.H{"name": name, "from": from, "to": to, "files": diffs})
	}
}

// loadVersionFiles downloads a version archive (promoted or pending) and returns path→content.
func loadVersionFiles(c *gin.Context, reg *registry.Registry, tenantID, name, version string) (map[string]string, error) {
	rc, err := reg.DownloadAny(c.Request.Context(), tenantID, name, version)
	if err != nil {
		return nil, err
	}
	defer rc.Close() //nolint:errcheck
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || strings.Contains(f.Name, "..") {
			continue
		}
		frc, err := f.Open()
		if err != nil {
			continue
		}
		b, err := io.ReadAll(frc)
		_ = frc.Close()
		if err != nil {
			continue
		}
		out[strings.TrimPrefix(f.Name, "./")] = string(b)
	}
	if len(out) == 0 {
		return nil, errors.New("empty archive")
	}
	return out, nil
}

// lineDiff computes a line-level diff between old and new text using an LCS table.
func lineDiff(oldText, newText string) []diffLine {
	var a, b []string
	if oldText != "" {
		a = strings.Split(oldText, "\n")
	}
	if newText != "" {
		b = strings.Split(newText, "\n")
	}

	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var out []diffLine
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			out = append(out, diffLine{Op: " ", Text: a[i]})
			i++
			j++
		} else if lcs[i+1][j] >= lcs[i][j+1] {
			out = append(out, diffLine{Op: "-", Text: a[i]})
			i++
		} else {
			out = append(out, diffLine{Op: "+", Text: b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, diffLine{Op: "-", Text: a[i]})
	}
	for ; j < m; j++ {
		out = append(out, diffLine{Op: "+", Text: b[j]})
	}
	return out
}
