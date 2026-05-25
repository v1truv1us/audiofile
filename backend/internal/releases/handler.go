package releases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	client         *http.Client
	musicBrainzURL string
	discogsURL     string
	discogsKey     string
	discogsSecret  string
}

type SearchResult struct {
	MBID     string `json:"mbid"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Year     *int   `json:"year,omitempty"`
	Label    string `json:"label,omitempty"`
	CoverURL string `json:"coverUrl,omitempty"`
}

type discogsSearch struct {
	Results []struct {
		ID          int      `json:"id"`
		Title       string   `json:"title"`
		Year        *int     `json:"year"`
		Label       []string `json:"label"`
		CoverImage  string   `json:"cover_image"`
		ResourceURL string   `json:"resource_url"`
	} `json:"results"`
}

type musicBrainzSearch struct {
	Releases []struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Date         string `json:"date"`
		ArtistCredit []struct {
			Name string `json:"name"`
		} `json:"artist-credit"`
		LabelInfo []struct {
			Label *struct {
				Name string `json:"name"`
			} `json:"label"`
		} `json:"label-info"`
	} `json:"releases"`
}

type searchCacheEntry struct {
	results []SearchResult
	expires time.Time
}

var searchCache = struct {
	sync.Mutex
	items map[string]searchCacheEntry
}{items: map[string]searchCacheEntry{}}

func NewHandler() *Handler {
	return &Handler{
		client:         &http.Client{Timeout: 6 * time.Second},
		musicBrainzURL: "https://musicbrainz.org",
		discogsURL:     "https://api.discogs.com",
		discogsKey:     os.Getenv("DISCOGS_CONSUMER_KEY"),
		discogsSecret:  os.Getenv("DISCOGS_CONSUMER_SECRET"),
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/search", h.search)
	r.Post("/scan", h.scan)
	return r
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "q is required", http.StatusBadRequest)
		return
	}

	key := strings.ToLower(q)
	now := time.Now()
	searchCache.Lock()
	entry, ok := searchCache.items[key]
	if ok && now.Before(entry.expires) {
		searchCache.Unlock()
		writeJSON(w, entry.results)
		return
	}
	searchCache.Unlock()

	results, ok := h.searchResults(w, r, q)
	if !ok {
		return
	}
	cacheResults(key, results, now)
	writeJSON(w, results)
}

func (h *Handler) scan(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Barcode string `json:"barcode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "barcode is required", http.StatusBadRequest)
		return
	}
	barcode := cleanBarcode(payload.Barcode)
	if barcode == "" {
		http.Error(w, "barcode is required", http.StatusBadRequest)
		return
	}
	results, ok := h.searchDiscogsBarcode(w, r, barcode)
	if !ok {
		return
	}
	writeJSON(w, map[string]any{"barcode": barcode, "results": results})
}

func (h *Handler) searchResults(w http.ResponseWriter, r *http.Request, q string) ([]SearchResult, bool) {
	if h.discogsKey != "" && h.discogsSecret != "" {
		results, ok := h.searchDiscogs(w, r, q)
		if !ok {
			return nil, false
		}
		if len(results) > 0 {
			return results, true
		}
	}

	return h.searchMusicBrainz(w, r, q)
}

func (h *Handler) searchDiscogs(w http.ResponseWriter, r *http.Request, q string) ([]SearchResult, bool) {
	return h.searchDiscogsWithParam(w, r, "q", q)
}

func (h *Handler) searchDiscogsBarcode(w http.ResponseWriter, r *http.Request, barcode string) ([]SearchResult, bool) {
	return h.searchDiscogsWithParam(w, r, "barcode", barcode)
}

func (h *Handler) searchDiscogsWithParam(w http.ResponseWriter, r *http.Request, param string, value string) ([]SearchResult, bool) {
	if h.discogsKey == "" || h.discogsSecret == "" {
		http.Error(w, "barcode scanning requires Discogs credentials", http.StatusServiceUnavailable)
		return nil, false
	}
	reqURL := h.discogsURL + "/database/search?type=release&per_page=8&" + param + "=" + url.QueryEscape(value) + "&key=" + url.QueryEscape(h.discogsKey) + "&secret=" + url.QueryEscape(h.discogsSecret)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, reqURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, false
	}
	req.Header.Set("User-Agent", "AudioFile/0.2.0 +https://audiofile.app")

	res, err := h.client.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) && netErr.Timeout() {
			http.Error(w, "Discogs search timed out. Try again in a moment or enter the release details manually.", http.StatusGatewayTimeout)
			return nil, false
		}
		return nil, true
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, true
	}

	var discogs discogsSearch
	if err := json.NewDecoder(res.Body).Decode(&discogs); err != nil {
		return nil, true
	}

	results := make([]SearchResult, 0, len(discogs.Results))
	for _, rel := range discogs.Results {
		artist, title := splitDiscogsTitle(rel.Title)
		result := SearchResult{
			MBID:     strconv.Itoa(rel.ID),
			Title:    title,
			Artist:   artist,
			Year:     rel.Year,
			Label:    firstString(rel.Label),
			CoverURL: rel.CoverImage,
		}
		results = append(results, result)
	}
	return results, true
}

func (h *Handler) searchMusicBrainz(w http.ResponseWriter, r *http.Request, q string) ([]SearchResult, bool) {
	// Build a structured query: if the input looks like "Artist - Title" or "Artist Title",
	// search for both artist and title fields. Otherwise do a broad search but boost exact matches.
	mbQuery := buildMBQuery(q)
	reqURL := h.musicBrainzURL + "/ws/2/release/?fmt=json&limit=8&query=" + url.QueryEscape(mbQuery)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, reqURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, false
	}
	req.Header.Set("User-Agent", "AudioFile/0.2.0 (https://github.com/v1truv1us/audiofile)")

	res, err := h.client.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) && netErr.Timeout() {
			http.Error(w, "MusicBrainz search timed out. Try again in a moment or enter the release details manually.", http.StatusGatewayTimeout)
			return nil, false
		}
		http.Error(w, "MusicBrainz search is unavailable. Enter the release details manually or try again later.", http.StatusBadGateway)
		return nil, false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		http.Error(w, "release search failed", http.StatusBadGateway)
		return nil, false
	}

	var mb musicBrainzSearch
	if err := json.NewDecoder(res.Body).Decode(&mb); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil, false
	}

	results := make([]SearchResult, 0, len(mb.Releases))
	for _, rel := range mb.Releases {
		result := SearchResult{
			MBID:     rel.ID,
			Title:    rel.Title,
			Artist:   artistName(rel.ArtistCredit),
			Year:     releaseYear(rel.Date),
			Label:    labelName(rel.LabelInfo),
			CoverURL: "https://coverartarchive.org/release/" + rel.ID + "/front-250",
		}
		results = append(results, result)
	}

	return results, true
}

func cacheResults(key string, results []SearchResult, now time.Time) {
	searchCache.Lock()
	searchCache.items[key] = searchCacheEntry{results: results, expires: now.Add(12 * time.Hour)}
	searchCache.Unlock()
}

func cleanBarcode(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func splitDiscogsTitle(value string) (string, string) {
	parts := strings.SplitN(value, " - ", 2)
	if len(parts) != 2 {
		return "", value
	}
	return parts[0], parts[1]
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func artistName(credits []struct {
	Name string `json:"name"`
}) string {
	parts := make([]string, 0, len(credits))
	for _, credit := range credits {
		if credit.Name != "" {
			parts = append(parts, credit.Name)
		}
	}
	return strings.Join(parts, "")
}

func labelName(labels []struct {
	Label *struct {
		Name string `json:"name"`
	} `json:"label"`
}) string {
	for _, info := range labels {
		if info.Label != nil && info.Label.Name != "" {
			return info.Label.Name
		}
	}
	return ""
}

func releaseYear(date string) *int {
	if len(date) < 4 {
		return nil
	}
	year := 0
	for _, ch := range date[:4] {
		if ch < '0' || ch > '9' {
			return nil
		}
		year = year*10 + int(ch-'0')
	}
	return &year
}

func buildMBQuery(q string) string {
	// Check for "Artist - Title" separator (explicit)
	if parts := strings.SplitN(q, " - ", 2); len(parts) == 2 {
		artist := strings.TrimSpace(parts[0])
		title := strings.TrimSpace(parts[1])
		if artist != "" && title != "" {
			return fmt.Sprintf("artist:\"%s\" AND release:\"%s\"", artist, title)
		}
	}

	// For short queries (1-2 words), use structured artist OR release search.
	// This handles self-titled albums like "Sublime" where the generic text search
	// buries the exact match among thousands of results.
	// For longer queries, use MusicBrainz's default relevance search which handles
	// "Miles Davis Kind of Blue" naturally.
	words := strings.Fields(q)
	if len(words) <= 2 {
		return fmt.Sprintf("artist:\"%s\" OR release:\"%s\"", q, q)
	}

	// Multi-word: let MusicBrainz do its relevance matching on the raw query
	return q
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}
