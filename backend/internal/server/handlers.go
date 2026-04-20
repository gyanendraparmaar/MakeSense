package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/gyanendraparmaar/makesense/backend/internal/storage"
)

// ------------------ Health ------------------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"model": s.cfg.GeminiModel,
	})
}

// ------------------ Analyze (JSON, one-shot) ------------------

type analyzeReq struct {
	Text   string `json:"text"`
	NoteID string `json:"note_id,omitempty"`
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req analyzeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		writeErr(w, http.StatusBadRequest, "text is required")
		return
	}

	// Cache lookup first. Same block hash => reuse.
	hash := storage.HashBlock(req.Text)
	if cached, err := s.store.GetCachedAnalysis(hash); err == nil && cached != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"type":       cached.BlockType,
			"confidence": cached.Confidence,
			"structured": cached.Structured,
			"model":      cached.Model,
			"cached":     true,
		})
		return
	}

	result, err := s.pipeline.Analyze(r.Context(), req.Text)
	if err != nil {
		log.Printf("analyze err: %v", err)
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}

	// Fire-and-forget cache write.
	if err := s.store.SaveAnalysis(req.NoteID, req.Text, string(result.Type), result.Model, result.Confidence, result.Structured); err != nil {
		log.Printf("cache save: %v", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"type":       result.Type,
		"confidence": result.Confidence,
		"structured": result.Structured,
		"model":      result.Model,
		"cached":     false,
	})
}

// ------------------ Analyze (SSE stream) ------------------
//
// We don't stream tokens from Gemini here (the one-shot JSON mode doesn't
// stream cleanly), but we DO stream pipeline stages so the UI can render
// progress: classified -> done. This is enough for a "live" feel and keeps
// the code simple for v0.

func (s *Server) handleAnalyzeStream(w http.ResponseWriter, r *http.Request) {
	var req analyzeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		writeErr(w, http.StatusBadRequest, "text is required")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // tell nginx/CDNs not to buffer

	send := func(event string, payload any) {
		data, _ := json.Marshal(payload)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	// Cache-hit fast path.
	hash := storage.HashBlock(req.Text)
	if cached, err := s.store.GetCachedAnalysis(hash); err == nil && cached != nil {
		send("classified", map[string]any{"type": cached.BlockType, "confidence": cached.Confidence})
		send("done", map[string]any{
			"type":       cached.BlockType,
			"confidence": cached.Confidence,
			"structured": cached.Structured,
			"model":      cached.Model,
			"cached":     true,
		})
		return
	}

	cls, err := s.pipeline.Classify(r.Context(), req.Text)
	if err != nil {
		send("error", map[string]any{"message": err.Error()})
		return
	}
	send("classified", map[string]any{"type": cls.Type, "confidence": cls.Confidence})

	// Reuse the classification — don't double-bill Gemini.
	result, err := s.pipeline.AnalyzeWith(r.Context(), req.Text, cls)
	if err != nil {
		send("error", map[string]any{"message": err.Error()})
		return
	}

	if err := s.store.SaveAnalysis(req.NoteID, req.Text, string(result.Type), result.Model, result.Confidence, result.Structured); err != nil {
		log.Printf("cache save: %v", err)
	}

	send("done", map[string]any{
		"type":       result.Type,
		"confidence": result.Confidence,
		"structured": result.Structured,
		"model":      result.Model,
		"cached":     false,
	})
}

// ------------------ Notes CRUD ------------------

type notePayload struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (s *Server) handleListNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := s.store.ListNotes()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, notes)
}

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	var p notePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	n, err := s.store.CreateNote(p.Title, p.Content)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	n, err := s.store.GetNote(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var p notePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	n, err := s.store.UpdateNote(id, p.Title, p.Content)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteNote(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ------------------ helpers ------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
