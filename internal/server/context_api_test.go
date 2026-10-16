package server

import (
	"encoding/json"
	"log-reviewer/internal/logs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAutomaticContextAPI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "context.log")
	if err := os.WriteFile(path, []byte("2026-10-14 12:29:00 INFO app : old\n2026-10-14 12:29:59 INFO app : requestId=abc before\n2026-10-14 12:30:00 ERROR app : requestId=abc failed\n2026-10-14 12:30:01 INFO app : requestId=other unrelated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s := New(dir)
	defer s.Close()
	var err error
	s.Report, err = logs.Analyze([]string{path}, dir, logs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	handler := s.Handler("127.0.0.1:8765")
	call := func(mode string) *httptest.ResponseRecorder {
		request := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/details?context="+mode, nil)
		request.Header.Set("Authorization", "Bearer "+s.Token())
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	for _, mode := range []string{"auto", "manual"} {
		response := call(mode)
		if response.Code != 200 {
			t.Fatal(response.Body.String())
		}
		var data struct {
			Details []logs.Detail `json:"details"`
		}
		if err = json.Unmarshal(response.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if len(data.Details) != 1 || data.Details[0].ContextMode != mode {
			t.Fatalf("%+v", data)
		}
		if mode == "auto" && (data.Details[0].ContextBefore != 1 || data.Details[0].ContextAfter != 0) {
			t.Fatal("automatic boundaries ignored")
		}
	}
	if call("invalid").Code != 400 {
		t.Fatal("invalid mode accepted")
	}
}
