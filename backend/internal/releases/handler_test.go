package releases

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRoutesRegistersSearch(t *testing.T) {
	h := NewHandler()
	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	h := NewHandler()
	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
	if !strings.Contains(res.Body.String(), "q is required") {
		t.Fatalf("expected missing query message, got %q", res.Body.String())
	}
}

func TestSearchUsesCachedResults(t *testing.T) {
	key := "cached query"
	cacheResults(key, []SearchResult{{MBID: "cached-id", Title: "Cached"}}, time.Now())
	h := &Handler{client: http.DefaultClient, musicBrainzURL: "http://unused.invalid"}
	req := httptest.NewRequest(http.MethodGet, "/search?q=cached+query", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "cached-id") {
		t.Fatalf("expected cached response, got status %d body %q", res.Code, res.Body.String())
	}
}

func TestSearchMapsMusicBrainzTimeoutToFriendlyGatewayTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	h := &Handler{
		client:         &http.Client{Timeout: 1 * time.Millisecond},
		musicBrainzURL: server.URL,
	}
	req := httptest.NewRequest(http.MethodGet, "/search?q=Until+The+Sun+Explodes", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusGatewayTimeout, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "MusicBrainz search timed out") {
		t.Fatalf("expected friendly timeout message, got %q", res.Body.String())
	}
	if strings.Contains(res.Body.String(), "context deadline exceeded") {
		t.Fatalf("expected raw Go timeout to be hidden, got %q", res.Body.String())
	}
}

func TestSearchMapsDiscogsTimeoutToFriendlyGatewayTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	h := &Handler{client: &http.Client{Timeout: time.Millisecond}, discogsURL: server.URL, discogsKey: "key", discogsSecret: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/search?q=slow-discogs", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusGatewayTimeout, res.Code, res.Body.String())
	}
}

func TestSearchUsesDiscogsBeforeMusicBrainz(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/database/search" {
			t.Fatalf("expected Discogs search path, got %q", r.URL.Path)
		}
		if r.URL.Query().Get("key") != "key" || r.URL.Query().Get("secret") != "secret" {
			t.Fatalf("expected Discogs credentials in query, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":123,"title":"Miles Davis - Kind of Blue","year":1959,"label":["Columbia"],"cover_image":"https://img.example/cover.jpg"}]}`))
	}))
	defer server.Close()

	h := &Handler{client: server.Client(), discogsURL: server.URL, musicBrainzURL: "http://unused.invalid", discogsKey: "key", discogsSecret: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/search?q=kind+of+blue", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{"123", "Kind of Blue", "Miles Davis", "1959", "Columbia", "https://img.example/cover.jpg"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response to contain %q, got %q", want, body)
		}
	}
}

func TestSearchFallsBackToMusicBrainzWhenDiscogsHasNoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/database/search" {
			_, _ = w.Write([]byte(`{"results":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"releases":[{"id":"abc","title":"Kind of Blue","date":"1959-08-17","artist-credit":[{"name":"Miles Davis"}],"label-info":[{"label":{"name":"Columbia"}}]}]}`))
	}))
	defer server.Close()

	h := &Handler{client: server.Client(), discogsURL: server.URL, musicBrainzURL: server.URL, discogsKey: "key", discogsSecret: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/search?q=empty-discogs", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "abc") {
		t.Fatalf("expected MusicBrainz fallback result, got %q", res.Body.String())
	}
}

func TestSearchFallsBackToMusicBrainzWhenDiscogsReturnsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/database/search" {
			_, _ = w.Write([]byte(`{`))
			return
		}
		_, _ = w.Write([]byte(`{"releases":[{"id":"fallback","title":"Fallback","date":"2000","artist-credit":[],"label-info":[]}]}`))
	}))
	defer server.Close()

	h := &Handler{client: server.Client(), discogsURL: server.URL, musicBrainzURL: server.URL, discogsKey: "key", discogsSecret: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/search?q=invalid-discogs-json", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "fallback") {
		t.Fatalf("expected fallback response, got status %d body %q", res.Code, res.Body.String())
	}
}

func TestSearchReturnsBadGatewayForMusicBrainzStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer server.Close()

	h := &Handler{client: server.Client(), musicBrainzURL: server.URL}
	req := httptest.NewRequest(http.MethodGet, "/search?q=mb-status", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, res.Code)
	}
}

func TestSearchReturnsBadGatewayForMusicBrainzInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{`)) }))
	defer server.Close()

	h := &Handler{client: server.Client(), musicBrainzURL: server.URL}
	req := httptest.NewRequest(http.MethodGet, "/search?q=mb-invalid-json", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, res.Code)
	}
}

func TestSearchParsesMusicBrainzResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); !strings.Contains(got, "CrateKeeper") {
			t.Fatalf("expected CrateKeeper user agent, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"releases":[{"id":"abc","title":"Kind of Blue","date":"1959-08-17","artist-credit":[{"name":"Miles Davis"}],"label-info":[{"label":{"name":"Columbia"}}]}]}`))
	}))
	defer server.Close()

	h := &Handler{client: server.Client(), musicBrainzURL: server.URL}
	req := httptest.NewRequest(http.MethodGet, "/search?q=musicbrainz-kind-of-blue", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{"Kind of Blue", "Miles Davis", "1959", "Columbia", "abc"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response to contain %q, got %q", want, body)
		}
	}
}

func TestDiscogsHelpersHandleMissingData(t *testing.T) {
	artist, title := splitDiscogsTitle("Untitled Bootleg")
	if artist != "" || title != "Untitled Bootleg" {
		t.Fatalf("expected missing artist separator to keep title, got artist=%q title=%q", artist, title)
	}
	if firstString(nil) != "" {
		t.Fatal("expected missing labels to return empty string")
	}
}

func TestLabelNameReturnsEmptyForMissingLabel(t *testing.T) {
	got := labelName([]struct {
		Label *struct {
			Name string `json:"name"`
		} `json:"label"`
	}{})
	if got != "" {
		t.Fatalf("expected empty label, got %q", got)
	}
}

func TestReleaseYear(t *testing.T) {
	year := releaseYear("1959-08-17")
	if year == nil || *year != 1959 {
		t.Fatalf("expected 1959, got %#v", year)
	}
	if releaseYear("bad-date") != nil {
		t.Fatal("expected invalid date to return nil")
	}
}
