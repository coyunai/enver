package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/coyunai/enver/internal/app"
	"github.com/coyunai/enver/internal/catalog"
	"github.com/coyunai/enver/internal/jobs"
	"github.com/coyunai/enver/internal/packages"
	"github.com/coyunai/enver/internal/preset"
	"github.com/coyunai/enver/internal/vfox"
)

type vfoxState struct {
	Available bool       `json:"available"`
	Info      *vfox.Info `json:"info,omitempty"`
	Error     string     `json:"error,omitempty"`
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.App.Overview(r.Context()))
}

func (s *Server) vfoxInfo(w http.ResponseWriter, _ *http.Request) {
	mgr, err := s.App.VfoxState()
	if err != nil {
		writeJSON(w, http.StatusOK, vfoxState{Error: err.Error()})
		return
	}
	info := mgr.Info()
	writeJSON(w, http.StatusOK, vfoxState{Available: true, Info: &info})
}

func (s *Server) setVfoxBin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Bin string `json:"bin"`
	}
	if err := decodeJSON(r, &body); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := s.App.SetVfoxBin(body.Bin); err != nil {
		apiError(w, http.StatusBadRequest, "vfox_unavailable", err.Error())
		return
	}
	s.vfoxInfo(w, r)
}

func (s *Server) pluginCatalog(w http.ResponseWriter, r *http.Request) {
	items, source := catalog.Load(r.Context(), catalog.CachePath(s.App.CacheDir), 12*time.Hour)
	installed := map[string]bool{}
	if mgr, err := s.App.VfoxState(); err == nil {
		for _, name := range mgr.PluginNames() {
			installed[name] = true
		}
	}
	for i := range items {
		items[i].Installed = installed[items[i].Name]
		delete(installed, items[i].Name)
	}
	for name := range installed {
		items = append(items, catalog.Plugin{Name: name, Installed: true, Desc: "本地安装的插件"})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	writeJSON(w, http.StatusOK, map[string]any{"source": source, "items": items})
}

func (s *Server) listSDKs(w http.ResponseWriter, r *http.Request) {
	sdks, err := s.App.SDKs(r.Context())
	if err != nil {
		apiError(w, http.StatusBadRequest, "sdk_list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sdks": sdks})
}

func (s *Server) addPlugin(w http.ResponseWriter, r *http.Request) {
	sdk := strings.TrimSpace(r.PathValue("sdk"))
	if sdk == "" {
		apiError(w, http.StatusBadRequest, "bad_request", "sdk is required")
		return
	}
	job, err := s.App.StartAddPlugin(sdk)
	if err != nil {
		apiError(w, http.StatusBadRequest, "plugin_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) versions(w http.ResponseWriter, r *http.Request) {
	sdk := strings.TrimSpace(r.PathValue("sdk"))
	versions, err := s.App.SearchVersions(r.Context(), sdk)
	if err != nil {
		apiError(w, http.StatusBadRequest, "search_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sdk": sdk, "versions": versions})
}

type versionRequest struct {
	SDK     string `json:"sdk"`
	Version string `json:"version"`
}

func (s *Server) install(w http.ResponseWriter, r *http.Request) {
	var body versionRequest
	if err := decodeJSON(r, &body); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	job, err := s.App.StartInstall(strings.TrimSpace(body.SDK), strings.TrimSpace(body.Version))
	if err != nil {
		apiError(w, http.StatusBadRequest, "install_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) uninstall(w http.ResponseWriter, r *http.Request) {
	var body versionRequest
	if err := decodeJSON(r, &body); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	job, err := s.App.StartUninstall(strings.TrimSpace(body.SDK), strings.TrimSpace(body.Version))
	if err != nil {
		apiError(w, http.StatusBadRequest, "uninstall_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) use(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SDK     string `json:"sdk"`
		Version string `json:"version"`
		Scope   string `json:"scope"`
		Dir     string `json:"dir"`
	}
	if err := decodeJSON(r, &body); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	logs := []string{}
	err := s.App.Use(r.Context(), body.SDK, body.Version, body.Scope, body.Dir, func(line string) {
		logs = append(logs, line)
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{"code": "use_failed", "message": err.Error()},
			"logs":  logs,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

func (s *Server) listPresets(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"presets": s.App.Presets.List()})
}

func (s *Server) savePreset(w http.ResponseWriter, r *http.Request) {
	var p preset.Preset
	if err := decodeJSON(r, &p); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if name := r.PathValue("name"); name != "" {
		p.Name = name
	}
	if err := s.App.Presets.Save(p); err != nil {
		apiError(w, http.StatusBadRequest, "preset_invalid", err.Error())
		return
	}
	saved, _ := s.App.Presets.Get(p.Name)
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) deletePreset(w http.ResponseWriter, r *http.Request) {
	if err := s.App.Presets.Delete(r.PathValue("name")); err != nil {
		apiError(w, http.StatusNotFound, "preset_not_found", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) applyPreset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Scope string `json:"scope"`
		Dir   string `json:"dir"`
	}
	_ = decodeJSON(r, &body)
	logs := []string{}
	err := s.App.ApplyPreset(r.Context(), r.PathValue("name"), body.Scope, body.Dir, func(line string) {
		logs = append(logs, line)
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{"code": "apply_failed", "message": err.Error()},
			"logs":  logs,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

func (s *Server) listVenvs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"venvs": s.App.VenvViews()})
}

func (s *Server) createVenv(w http.ResponseWriter, r *http.Request) {
	var req app.CreateVenvRequest
	if err := decodeJSON(r, &req); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	logs := []string{}
	record, err := s.App.CreateVenv(r.Context(), req, func(line string) { logs = append(logs, line) })
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{"code": "venv_failed", "message": err.Error()},
			"logs":  logs,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"venv": record, "logs": logs})
}

func (s *Server) deleteVenv(w http.ResponseWriter, r *http.Request) {
	if err := s.App.DeleteVenv(r.PathValue("id")); err != nil {
		apiError(w, http.StatusBadRequest, "venv_delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) rebuildVenv(w http.ResponseWriter, r *http.Request) {
	logs := []string{}
	record, err := s.App.RebuildVenv(r.Context(), r.PathValue("id"), func(line string) { logs = append(logs, line) })
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{"code": "venv_rebuild_failed", "message": err.Error()},
			"logs":  logs,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"venv": record, "logs": logs})
}

func (s *Server) listPackages(w http.ResponseWriter, r *http.Request) {
	sdk := r.URL.Query().Get("sdk")
	version := r.URL.Query().Get("version")
	if sdk == "" {
		apiError(w, http.StatusBadRequest, "bad_request", "sdk is required")
		return
	}
	list, err := s.App.Packages(r.Context(), sdk, version)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, packages.ErrUnsupported) {
			status = http.StatusNotImplemented
		}
		apiError(w, status, "packages_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"packages": list})
}

func (s *Server) exportPackages(w http.ResponseWriter, r *http.Request) {
	sdk := r.URL.Query().Get("sdk")
	version := r.URL.Query().Get("version")
	if sdk == "" {
		apiError(w, http.StatusBadRequest, "bad_request", "sdk is required")
		return
	}
	export, err := s.App.ExportPackages(r.Context(), sdk, version)
	if err != nil {
		apiError(w, http.StatusBadRequest, "export_failed", err.Error())
		return
	}
	contentType := "text/plain; charset=utf-8"
	if strings.HasSuffix(export.Filename, ".json") {
		contentType = "application/json; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", export.Filename))
	_, _ = w.Write([]byte(export.Content))
}

func (s *Server) doctor(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.App.Doctor())
}

func (s *Server) storage(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.App.Storage())
}

func (s *Server) setStorage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SDKPath string `json:"sdkPath"`
	}
	if err := decodeJSON(r, &body); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := s.App.SetStoragePath(r.Context(), body.SDKPath); err != nil {
		apiError(w, http.StatusBadRequest, "storage_failed", err.Error())
		return
	}
	s.storage(w, r)
}

func (s *Server) migrateStorage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path      string `json:"path"`
		DeleteOld bool   `json:"deleteOld"`
	}
	if err := decodeJSON(r, &body); err != nil {
		apiError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	job, err := s.App.StartMigrate(r.Context(), body.Path, body.DeleteOld)
	if err != nil {
		apiError(w, http.StatusBadRequest, "migrate_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) listJobs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"jobs": s.App.Jobs.List()})
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	job, ok := s.App.Jobs.Get(r.PathValue("id"))
	if !ok {
		apiError(w, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job.Snapshot())
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	job, ok := s.App.Jobs.Get(r.PathValue("id"))
	if !ok {
		apiError(w, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	job.Cancel()
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) jobEvents(w http.ResponseWriter, r *http.Request) {
	job, ok := s.App.Jobs.Get(r.PathValue("id"))
	if !ok {
		apiError(w, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		apiError(w, http.StatusInternalServerError, "stream_unsupported", "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	snapshot := job.Snapshot()
	history, ch := job.Subscribe()
	send := func(e jobs.Event) bool {
		data, err := json.Marshal(e)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	for _, e := range history {
		if !send(e) {
			return
		}
	}
	if snapshot.Status != jobs.StatusRunning {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, open := <-ch:
			if !open {
				return
			}
			if !send(e) {
				return
			}
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
