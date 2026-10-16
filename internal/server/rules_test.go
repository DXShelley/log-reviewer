package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"log-reviewer/internal/logs"
)

func TestLanguageRuleAPI(t *testing.T) {
	s := New(t.TempDir())
	handler := s.Handler("127.0.0.1:8765")
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://127.0.0.1:8765"+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+s.Token())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	response := call("GET", "/api/language-rules", "")
	if response.Code != 200 {
		t.Fatal(response.Code)
	}
	var data struct {
		Languages []logs.LanguageEntry `json:"languages"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil || len(data.Languages) != 17 {
		t.Fatal(err)
	}
	body := `{"id":"java","evidence":"MY_JAVA_FAILURE","start":"^MY_JAVA_FAILURE:","level":"ERROR"}`
	if response = call("PUT", "/api/language-rules", body); response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	if response = call("PUT", "/api/language-rules", `{"id":"java","evidence":"["}`); response.Code != 400 {
		t.Fatal(response.Code)
	}
	saved, err := s.rules.Overrides()
	if err != nil || len(saved) != 1 || saved[0].Evidence != "MY_JAVA_FAILURE" {
		t.Fatal(saved, err)
	}
	if response = call("DELETE", "/api/language-rules?id=java", ""); response.Code != 200 {
		t.Fatal(response.Code)
	}
	saved, err = s.rules.Overrides()
	if err != nil || len(saved) != 0 {
		t.Fatal(saved, err)
	}
	if response = call("DELETE", "/api/language-rules?id=..%2Fescape", ""); response.Code != 400 {
		t.Fatal(response.Code)
	}
}
