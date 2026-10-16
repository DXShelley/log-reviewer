package server

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChineseImportAPIs(t *testing.T) {
	raw := "2026-10-16 12:00:00 ERROR demo : " + string([]byte{0xb5, 0xbc, 0xb3, 0xf6, 0xbd, 0xe1, 0xca, 0xf8}) + "\n"
	for _, mode := range []string{"path", "directory", "upload"} {
		t.Run(mode, func(t *testing.T) {
			s := New(t.TempDir())
			defer s.Close()
			handler := s.Handler("127.0.0.1:8765")
			call := func(method, route, body string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(method, "http://127.0.0.1:8765/api/"+route, strings.NewReader(body))
				req.Header.Set("Authorization", "Bearer "+s.Token())
				result := httptest.NewRecorder()
				handler.ServeHTTP(result, req)
				if result.Code != 200 {
					t.Fatalf("%s: %d %s", route, result.Code, result.Body.String())
				}
				return result
			}
			var body []byte
			if mode == "upload" {
				response := call("POST", "upload?name=export.log", raw)
				var upload struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &upload); err != nil || upload.ID == "" {
					t.Fatal(response.Body.String())
				}
				body, _ = json.Marshal(map[string]any{"uploads": []string{upload.ID}})
			} else {
				dir := t.TempDir()
				name := filepath.Join(dir, "export.log")
				if err := os.WriteFile(name, []byte(raw), 0600); err != nil {
					t.Fatal(err)
				}
				if mode == "directory" {
					name = dir
				}
				selection, _ := json.Marshal(map[string]any{"paths": []string{name}})
				response := call("POST", "paths", string(selection))
				var paths struct {
					Paths []string `json:"paths"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &paths); err != nil {
					t.Fatal(err)
				}
				body, _ = json.Marshal(map[string]any{"paths": paths.Paths})
			}
			call("POST", "analyze", string(body))
			s.jobs.Wait()
			for _, route := range []string{"groups", "details?keyword=" + url.QueryEscape("导出结束"), "export?keyword=" + url.QueryEscape("导出结束")} {
				response := call("GET", route, "")
				if !strings.Contains(response.Body.String(), "导出结束") || strings.Contains(response.Body.String(), "�") {
					t.Fatalf("%s: %s", route, response.Body.String())
				}
			}
		})
	}
}
