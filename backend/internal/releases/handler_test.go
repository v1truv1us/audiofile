package releases

import (
	"context"
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

func TestRoutesRegistersScan(t *testing.T) {
	h := NewHandler()
	req := httptest.NewRequest(http.MethodPost, "/scan", nil)
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestScanSearchesDiscogsByBarcode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/database/search" {
			t.Fatalf("expected Discogs search path, got %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("barcode"); got != "018771210510" {
			t.Fatalf("expected barcode query, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":777,"title":"Sublime - 40oz. To Freedom","year":1992,"label":["Skunk Records"],"cover_image":"https://img.example/sublime.jpg"}]}`))
	}))
	defer server.Close()

	h := &Handler{discogs: NewDiscogsClient(server.URL, "discogs-key", "discogs-secret")}
	req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(`{"barcode":"018771210510"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	h.scan(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	for _, want := range []string{"018771210510", "40oz. To Freedom", "Skunk Records", "https://img.example/sublime.jpg"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("expected scan response to contain %q, got %q", want, res.Body.String())
		}
	}
}

func TestScanRequiresBarcode(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(`{}`))
	res := httptest.NewRecorder()

	h.scan(res, req)

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
	h := &Handler{searchers: []ReleaseSearcher{NewMusicBrainzClient("http://unused.invalid")}}
	req := httptest.NewRequest(http.MethodGet, "/search?q=cached+query", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "cached-id") {
		t.Fatalf("expected cached response, got status %d body %q", res.Code, res.Body.String())
	}
}

func TestSearchReturnsBadGatewayWhenAllSearchersFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	h := &Handler{
		searchers: []ReleaseSearcher{&MusicBrainzClient{client: &http.Client{Timeout: 1 * time.Millisecond}, baseURL: server.URL}},
	}
	req := httptest.NewRequest(http.MethodGet, "/search?q=Until+The+Sun+Explodes", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusBadGateway, res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "context deadline exceeded") {
		t.Fatalf("expected friendly message, got raw error: %q", res.Body.String())
	}
	if strings.Contains(res.Body.String(), "context deadline exceeded") {
		t.Fatalf("expected raw Go timeout to be hidden, got %q", res.Body.String())
	}
}

func TestSearchFallsThroughWhenSearcherTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	h := &Handler{discogs: NewDiscogsClient(server.URL, "key", "secret"), searchers: []ReleaseSearcher{NewDiscogsClient(server.URL, "key", "secret")}}
	req := httptest.NewRequest(http.MethodGet, "/search?q=slow-discogs", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	// Discogs times out, falls through to MusicBrainz which is also unavailable
	if res.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusBadGateway, res.Code, res.Body.String())
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

	h := &Handler{discogs: NewDiscogsClient(server.URL, "key", "secret"), searchers: []ReleaseSearcher{NewDiscogsClient(server.URL, "key", "secret"), NewMusicBrainzClient("http://unused.invalid")}}
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

	h := &Handler{discogs: NewDiscogsClient(server.URL, "key", "secret"), searchers: []ReleaseSearcher{NewDiscogsClient(server.URL, "key", "secret"), NewMusicBrainzClient(server.URL)}}
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

	h := &Handler{discogs: NewDiscogsClient(server.URL, "key", "secret"), searchers: []ReleaseSearcher{NewDiscogsClient(server.URL, "key", "secret"), NewMusicBrainzClient(server.URL)}}
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

	h := &Handler{searchers: []ReleaseSearcher{NewMusicBrainzClient(server.URL)}}
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

	h := &Handler{searchers: []ReleaseSearcher{NewMusicBrainzClient(server.URL)}}
	req := httptest.NewRequest(http.MethodGet, "/search?q=mb-invalid-json", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, res.Code)
	}
}

func TestSearchParsesMusicBrainzResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); !strings.Contains(got, "AudioFile") {
			t.Fatalf("expected AudioFile user agent, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"releases":[{"id":"abc","title":"Kind of Blue","date":"1959-08-17","artist-credit":[{"name":"Miles Davis"}],"label-info":[{"label":{"name":"Columbia"}}]}]}`))
	}))
	defer server.Close()

	h := &Handler{searchers: []ReleaseSearcher{NewMusicBrainzClient(server.URL)}}
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

func TestSetDiscogsConfiguresBarcodeClient(t *testing.T) {
	d := NewDiscogsClient("https://discogs.example", "key", "secret")
	h := NewHandler()

	h.SetDiscogs(d)

	if h.discogs != d {
		t.Fatal("expected SetDiscogs to store client")
	}
}

func TestNewDiscogsClientReturnsNilWithoutCredentials(t *testing.T) {
	if got := NewDiscogsClient("https://discogs.example", "", "secret"); got != nil {
		t.Fatalf("expected nil client without key, got %#v", got)
	}
	if got := NewDiscogsClient("https://discogs.example", "key", ""); got != nil {
		t.Fatalf("expected nil client without secret, got %#v", got)
	}
}

func TestNewDiscogsClientUsesDefaultBaseURL(t *testing.T) {
	got := NewDiscogsClient("", "key", "secret")
	if got == nil {
		t.Fatal("expected Discogs client")
	}
	if got.baseURL != "https://api.discogs.com" {
		t.Fatalf("expected default base URL, got %q", got.baseURL)
	}
}

func TestNewMusicBrainzClientUsesDefaultBaseURL(t *testing.T) {
	got := NewMusicBrainzClient("")
	if got.baseURL != "https://musicbrainz.org" {
		t.Fatalf("expected default base URL, got %q", got.baseURL)
	}
}

func TestMusicBrainzSearchBarcodeIsUnsupported(t *testing.T) {
	got, err := NewMusicBrainzClient("http://unused.invalid").SearchBarcode(context.Background(), "123456")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil results, got %#v", got)
	}
}

func TestDiscogsSearchBarcodeReturnsEmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("barcode"); got != "000111222333" {
			t.Fatalf("expected barcode query, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()

	got, err := NewDiscogsClient(server.URL, "key", "secret").SearchBarcode(context.Background(), "000111222333")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty results, got %#v", got)
	}
}

func TestDiscogsSearchBarcodeReturnsDecodeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{`))
	}))
	defer server.Close()

	_, err := NewDiscogsClient(server.URL, "key", "secret").SearchBarcode(context.Background(), "000111222333")
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestDiscogsSearchReturnsStatusErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := NewDiscogsClient(server.URL, "key", "secret").Search(context.Background(), "rate limited")
	if err == nil {
		t.Fatal("expected status error")
	}
}

func TestScanRejectsInvalidJSON(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(`{`))
	res := httptest.NewRecorder()

	h.scan(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestScanRequiresDiscogsCredentials(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(`{"barcode":"018771210510"}`))
	res := httptest.NewRecorder()

	h.scan(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, res.Code)
	}
}

func TestScanReturnsBadGatewayWhenDiscogsFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	h := &Handler{discogs: NewDiscogsClient(server.URL, "key", "secret")}
	req := httptest.NewRequest(http.MethodPost, "/scan", strings.NewReader(`{"barcode":"018771210510"}`))
	res := httptest.NewRecorder()

	h.scan(res, req)

	if res.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, res.Code)
	}
}

func TestSearchReturnsEmptyArrayWhenSearchersHaveNoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"releases":[]}`))
	}))
	defer server.Close()

	h := &Handler{searchers: []ReleaseSearcher{NewMusicBrainzClient(server.URL)}}
	req := httptest.NewRequest(http.MethodGet, "/search?q=no-results-unique", nil)
	res := httptest.NewRecorder()

	h.search(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	if strings.TrimSpace(res.Body.String()) != "null" {
		t.Fatalf("expected null empty result response, got %q", res.Body.String())
	}
}

func TestBuildMBQuery(t *testing.T) {
	if got := buildMBQuery("Miles Davis - Kind of Blue"); got != `artist:"Miles Davis" AND release:"Kind of Blue"` {
		t.Fatalf("expected split artist/title query, got %q", got)
	}
	if got := buildMBQuery("Sade"); got != `artist:"Sade" OR release:"Sade"` {
		t.Fatalf("expected short query expansion, got %q", got)
	}
	if got := buildMBQuery("The Shape Of Jazz To Come"); got != "The Shape Of Jazz To Come" {
		t.Fatalf("expected long query unchanged, got %q", got)
	}
}

func TestArtistNameJoinsNonEmptyCredits(t *testing.T) {
	got := artistName([]struct {
		Name string `json:"name"`
	}{{Name: "Miles"}, {Name: ""}, {Name: "Davis"}})
	if got != "MilesDavis" {
		t.Fatalf("expected joined artist credits, got %q", got)
	}
}

func TestLabelNameSkipsNilAndEmptyLabels(t *testing.T) {
	got := labelName([]struct {
		Label *struct {
			Name string `json:"name"`
		} `json:"label"`
	}{
		{},
		{Label: &struct {
			Name string `json:"name"`
		}{Name: ""}},
		{Label: &struct {
			Name string `json:"name"`
		}{Name: "Columbia"}},
	})
	if got != "Columbia" {
		t.Fatalf("expected first non-empty label, got %q", got)
	}
}

func TestReleaseYearReturnsNilForShortDate(t *testing.T) {
	if got := releaseYear("199"); got != nil {
		t.Fatalf("expected nil for short date, got %#v", got)
	}
}
