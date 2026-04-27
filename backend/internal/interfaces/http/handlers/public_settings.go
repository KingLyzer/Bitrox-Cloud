package handlers

import (
	"context"
	"encoding/json"
	"net/http"
)

type PublicSettingsRepository interface {
	List(ctx context.Context) ([]AdminSettingRecord, error)
}

type PublicSettingsHandler struct {
	repo PublicSettingsRepository
}

func NewPublicSettingsHandler(repo PublicSettingsRepository) *PublicSettingsHandler {
	return &PublicSettingsHandler{repo: repo}
}

func (h *PublicSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	sections := defaultAdminSettingsSections()

	if h.repo != nil {
		items, err := h.repo.List(r.Context())
		if err != nil {
			writeInternalOrSchemaError(w, err, "failed to load public settings")
			return
		}
		for _, item := range items {
			current, ok := sections[item.Key]
			if !ok {
				continue
			}
			currentMap, ok := current.(map[string]any)
			if !ok {
				continue
			}
			persisted := map[string]any{}
			if err := json.Unmarshal(item.ValueJSON, &persisted); err != nil {
				continue
			}
			for key, value := range persisted {
				currentMap[key] = value
			}
			sections[item.Key] = currentMap
		}
	}

	general, _ := sections["general"].(map[string]any)

	writeJSON(w, http.StatusOK, map[string]any{
		"site_name":        stringValue(general["site_name"], "BitroxCloud"),
		"site_subtitle":    stringValue(general["site_subtitle"], "Private storage"),
		"browser_title":    stringValue(general["browser_title"], "BitroxCloud"),
		"logo_url":         stringValue(general["site_logo_url"], "https://pb.dashboardicons.com/api/files/community_gallery/myyy4r7vdmreido/bitrocloud_ameyfihhth.png"),
		"favicon_url":      stringValue(general["favicon_url"], ""),
		"accent_color":     stringValue(general["brand_color"], "#2563eb"),
		"public_base_url":  stringValue(general["public_base_url"], ""),
		"default_language": stringValue(general["default_language"], "en"),
		"timezone":         stringValue(general["default_timezone"], "UTC"),
		"maintenance_mode": boolValue(general["maintenance_mode"], false),
	})
}

func stringValue(value any, fallback string) string {
	raw, ok := value.(string)
	if !ok {
		return fallback
	}
	if raw == "" {
		return fallback
	}
	return raw
}

func boolValue(value any, fallback bool) bool {
	raw, ok := value.(bool)
	if !ok {
		return fallback
	}
	return raw
}
