package server

import (
	"encoding/json"
	"net/http"

	"log-reviewer/internal/logs"
)

func (s *Server) UseRuleDirectory(dir string) error {
	store := &logs.RuleStore{Dir: dir}
	if _, err := store.Overrides(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = store
	return nil
}
func (s *Server) languageRules(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	store := s.rules
	s.mu.RUnlock()
	switch r.Method {
	case "GET":
		entries, err := store.Catalogue()
		if err != nil {
			fail(w, 500, err)
			return
		}
		output(w, map[string]any{"languages": entries})
	case "PUT":
		var rule logs.LanguageRule
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&rule); err != nil {
			fail(w, 400, err)
			return
		}
		if err := logs.ValidateLanguageRule(rule); err != nil {
			fail(w, 400, err)
			return
		}
		if err := store.Save(rule); err != nil {
			fail(w, 500, err)
			return
		}
		output(w, map[string]bool{"saved": true})
	case "DELETE":
		if err := store.Reset(r.URL.Query().Get("id")); err != nil {
			fail(w, 400, err)
			return
		}
		output(w, map[string]bool{"reset": true})
	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		w.WriteHeader(405)
	}
}
