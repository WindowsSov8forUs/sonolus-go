package devserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/build"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler"
)

func TestNewServesMultipleDevelopmentLevels(t *testing.T) {
	handler, err := New("test", &compiler.Artifacts{}, &build.PackagedEngine{}, []Level{
		{Name: "basic", Title: "Basic Level", Data: []byte("basic")},
		{Name: "alternate", Title: "Alternate Level", Data: []byte("alternate")},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sonolus/levels/list", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 2 || response.Items[0].Name != "basic" || response.Items[1].Name != "alternate" {
		t.Fatalf("items = %#v", response.Items)
	}
}

func TestNewRejectsDuplicateDevelopmentLevelNames(t *testing.T) {
	_, err := New("test", &compiler.Artifacts{}, &build.PackagedEngine{}, []Level{
		{Name: "duplicate", Title: "First", Data: []byte("first")},
		{Name: "duplicate", Title: "Second", Data: []byte("second")},
	})
	if err == nil {
		t.Fatal("duplicate development level names succeeded")
	}
}
