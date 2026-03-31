package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/ntwine-ai/ntwine/internal/config"
	"github.com/ntwine-ai/ntwine/internal/harness"
	"github.com/ntwine-ai/ntwine/internal/openrouter"
	"github.com/ntwine-ai/ntwine/internal/orchestrator"
)

func NewRouter(frontend http.Handler, registry *harness.Registry) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/config", handleGetConfig)
	mux.HandleFunc("POST /api/config", handleSetConfig)
	mux.HandleFunc("POST /api/config/models", handleAddModel)
	mux.HandleFunc("DELETE /api/config/models/{id...}", handleRemoveModel)
	mux.HandleFunc("GET /api/providers", handleListProviders)
	mux.HandleFunc("GET /api/discuss", makeDiscussHandler(registry))
	mux.HandleFunc("GET /api/history", handleListHistory)
	mux.HandleFunc("GET /api/history/{id}", handleGetHistory)
	mux.HandleFunc("DELETE /api/history/{id}", handleDeleteHistory)
	mux.HandleFunc("GET /api/export/{id}", handleExport)

	if frontend != nil {
		mux.Handle("/", frontend)
	}

	return mux
}

func maskKey(raw string) string {
	if len(raw) <= 8 {
		return "***"
	}
	return raw[:5] + "..." + raw[len(raw)-3:]
}

func handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	maskedKeys := make(map[string]string, len(cfg.ProviderKeys))
	keysSet := make([]string, 0, len(cfg.ProviderKeys))
	for k, v := range cfg.ProviderKeys {
		if v != "" {
			maskedKeys[k] = maskKey(v)
			keysSet = append(keysSet, k)
		}
	}
	if cfg.APIKey != "" {
		if _, exists := maskedKeys["openrouter"]; !exists {
			maskedKeys["openrouter"] = maskKey(cfg.APIKey)
			keysSet = append(keysSet, "openrouter")
		}
	}
	tavilyMasked := ""
	if cfg.TavilyKey != "" {
		tavilyMasked = maskKey(cfg.TavilyKey)
		keysSet = append(keysSet, "_tavily")
		maskedKeys["_tavily"] = tavilyMasked
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"models":            cfg.Models,
		"masked_keys":       maskedKeys,
		"provider_keys_set": keysSet,
		"tavily_masked":     tavilyMasked,
	})
}

func handleSetConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		APIKey          *string            `json:"api_key"`
		Models          []string           `json:"models"`
		TavilyKey       *string            `json:"tavily_api_key"`
		ProviderKeys    map[string]string  `json:"provider_keys"`
		AddProviderKey  *struct {
			Provider string `json:"provider"`
			Key      string `json:"key"`
		} `json:"add_provider_key"`
		RemoveProviderKey *string `json:"remove_provider_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated := cfg

	if body.AddProviderKey != nil {
		provider := body.AddProviderKey.Provider
		key := body.AddProviderKey.Key
		if provider == "_tavily" {
			updated = config.Config{APIKey: updated.APIKey, Models: updated.Models, TavilyKey: key, ProviderKeys: updated.ProviderKeys}
		} else {
			newKeys := make(map[string]string, len(updated.ProviderKeys)+1)
			for k, v := range updated.ProviderKeys {
				newKeys[k] = v
			}
			newKeys[provider] = key
			if provider == "openrouter" {
				updated = config.Config{APIKey: key, Models: updated.Models, TavilyKey: updated.TavilyKey, ProviderKeys: newKeys}
			} else {
				updated = config.Config{APIKey: updated.APIKey, Models: updated.Models, TavilyKey: updated.TavilyKey, ProviderKeys: newKeys}
			}
		}
	}

	if body.RemoveProviderKey != nil {
		provider := *body.RemoveProviderKey
		if provider == "_tavily" {
			updated = config.Config{APIKey: updated.APIKey, Models: updated.Models, TavilyKey: "", ProviderKeys: updated.ProviderKeys}
		} else {
			newKeys := make(map[string]string, len(updated.ProviderKeys))
			for k, v := range updated.ProviderKeys {
				if k != provider {
					newKeys[k] = v
				}
			}
			if provider == "openrouter" {
				updated = config.Config{APIKey: "", Models: updated.Models, TavilyKey: updated.TavilyKey, ProviderKeys: newKeys}
			} else {
				updated = config.Config{APIKey: updated.APIKey, Models: updated.Models, TavilyKey: updated.TavilyKey, ProviderKeys: newKeys}
			}
		}
	}

	if body.APIKey != nil {
		updated = config.Config{APIKey: *body.APIKey, Models: updated.Models, TavilyKey: updated.TavilyKey, ProviderKeys: updated.ProviderKeys}
	}
	if body.Models != nil {
		updated = config.Config{APIKey: updated.APIKey, Models: body.Models, TavilyKey: updated.TavilyKey, ProviderKeys: updated.ProviderKeys}
	}
	if body.TavilyKey != nil {
		updated = config.Config{APIKey: updated.APIKey, Models: updated.Models, TavilyKey: *body.TavilyKey, ProviderKeys: updated.ProviderKeys}
	}
	if body.ProviderKeys != nil {
		merged := make(map[string]string, len(updated.ProviderKeys)+len(body.ProviderKeys))
		for k, v := range updated.ProviderKeys {
			merged[k] = v
		}
		for k, v := range body.ProviderKeys {
			if v == "" {
				delete(merged, k)
			} else {
				merged[k] = v
			}
		}
		updated = config.Config{APIKey: updated.APIKey, Models: updated.Models, TavilyKey: updated.TavilyKey, ProviderKeys: merged}
	}

	if err := config.Save(updated); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	maskedResp := make(map[string]string, len(updated.ProviderKeys))
	keysSet := make([]string, 0, len(updated.ProviderKeys))
	for k, v := range updated.ProviderKeys {
		if v != "" {
			maskedResp[k] = maskKey(v)
			keysSet = append(keysSet, k)
		}
	}
	tavilyMasked := ""
	if updated.TavilyKey != "" {
		tavilyMasked = maskKey(updated.TavilyKey)
		keysSet = append(keysSet, "_tavily")
		maskedResp["_tavily"] = tavilyMasked
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"models":            updated.Models,
		"masked_keys":       maskedResp,
		"provider_keys_set": keysSet,
		"tavily_masked":     tavilyMasked,
	})
}

func handleAddModel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated := config.AddModel(cfg, body.Model)
	if err := config.Save(updated); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"models": updated.Models})
}

func handleRemoveModel(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("id")
	if modelID == "" {
		writeError(w, http.StatusBadRequest, "model id is required")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated := config.RemoveModel(cfg, modelID)
	if err := config.Save(updated); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"models": updated.Models})
}

func handleListProviders(w http.ResponseWriter, r *http.Request) {
	type providerInfo struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	providers := make([]providerInfo, 0, len(openrouter.AllProviders))
	for _, p := range openrouter.AllProviders {
		providers = append(providers, providerInfo{ID: p.ID, Name: p.Name})
	}
	writeJSON(w, http.StatusOK, providers)
}

type discussRequest struct {
	Prompt       string `json:"prompt"`
	CodebasePath string `json:"codebase_path"`
	Rounds       int    `json:"rounds"`
}

type clientMessage struct {
	Action  string `json:"action"`
	ModelID string `json:"model_id,omitempty"`
	Content string `json:"content,omitempty"`
}

func makeDiscussHandler(registry *harness.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handleDiscuss(w, r, registry)
	}
}

func handleDiscuss(w http.ResponseWriter, r *http.Request, registry *harness.Registry) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		http.Error(w, "websocket accept failed", http.StatusInternalServerError)
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1024 * 1024)

	log.Printf("[WS] connection accepted")
	ctx := r.Context()

	var req discussRequest
	if err := wsjson.Read(ctx, conn, &req); err != nil {
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		wsjson.Write(ctx, conn, orchestrator.Event{Type: "error", Content: "prompt is required"})
		return
	}
	if strings.TrimSpace(req.CodebasePath) == "" {
		wsjson.Write(ctx, conn, orchestrator.Event{Type: "error", Content: "codebase_path is required"})
		return
	}

	cfg, err := config.Load()
	if err != nil {
		wsjson.Write(ctx, conn, orchestrator.Event{Type: "error", Content: "failed to load config: " + err.Error()})
		return
	}
	providerKeys := config.BuildProviderKeys(cfg)
	if len(providerKeys) == 0 {
		wsjson.Write(ctx, conn, orchestrator.Event{Type: "error", Content: "no API keys configured"})
		return
	}
	if len(cfg.Models) == 0 {
		wsjson.Write(ctx, conn, orchestrator.Event{Type: "error", Content: "no models configured"})
		return
	}

	client := openrouter.NewClient(providerKeys)
	discCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	mutes := orchestrator.NewMuteSet()
	pins := orchestrator.NewPinSet()
	injector := orchestrator.NewInjector()

	var writeMu sync.Mutex
	broadcast := func(event orchestrator.Event) {
		writeMu.Lock()
		defer writeMu.Unlock()
		if err := wsjson.Write(discCtx, conn, event); err != nil {
			log.Printf("[WS] write error: %v", err)
			cancel()
		}
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-discCtx.Done():
				return
			case <-ticker.C:
				if err := conn.Ping(discCtx); err != nil {
					log.Printf("[WS] ping failed: %v", err)
					cancel()
					return
				}
			}
		}
	}()

	go func() {
		for {
			var msg clientMessage
			if err := wsjson.Read(ctx, conn, &msg); err != nil {
				log.Printf("[WS] read error (client disconnect?): %v", err)
				cancel()
				return
			}
			switch msg.Action {
			case "stop":
				log.Printf("[WS] client requested stop")
				cancel()
				return
			case "mute":
				log.Printf("[WS] muting model: %s", msg.ModelID)
				mutes.Mute(msg.ModelID)
			case "unmute":
				log.Printf("[WS] unmuting model: %s", msg.ModelID)
				mutes.Unmute(msg.ModelID)
			case "inject":
				log.Printf("[WS] god injection: %s", msg.Content)
				injector.Send(msg.Content)
			}
		}
	}()

	discID := fmt.Sprintf("disc_%d", time.Now().UnixMilli())
	disc := orchestrator.NewDiscussion(discID, req.Prompt, req.CodebasePath, cfg.Models, req.Rounds)
	result := orchestrator.Run(discCtx, disc, client, registry, broadcast, mutes, pins, injector)

	record := orchestrator.BuildRecord(disc, result)
	if err := config.SaveDiscussion(record); err != nil {
		log.Printf("[WS] failed to save discussion history: %v", err)
	}

	log.Printf("[WS] discussion complete, closing connection")
	conn.Close(websocket.StatusNormalClosure, "discussion complete")
}

func handleListHistory(w http.ResponseWriter, r *http.Request) {
	summaries, err := config.LoadDiscussions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summaries)
}

func handleGetHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "discussion id is required")
		return
	}
	disc, err := config.LoadDiscussion(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, disc)
}

func handleDeleteHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "discussion id is required")
		return
	}
	if err := config.DeleteDiscussion(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
