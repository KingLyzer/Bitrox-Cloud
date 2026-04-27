package handlers

import (
	"net/http"
	"time"
)

type MetaHandler struct {
	version string
}

func NewMetaHandler(version string) *MetaHandler {
	return &MetaHandler{version: version}
}

func (h *MetaHandler) Version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version": h.version,
		"time":    time.Now().UTC(),
	})
}
