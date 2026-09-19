// perfcheck boots the backend against a throwaway database and measures the
// wire cost of the content endpoints end to end — gzip sizes, slim shapes,
// version-signal behaviour, delta fetches. Run from the repo root:
//
//	go run ./tools/perfcheck
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ap-psych-final/backend/backend"
)

var base = "http://127.0.0.1:18099"

func main() {
	dir, err := os.MkdirTemp("", "perfcheck")
	must(err)
	defer os.RemoveAll(dir)

	app, err := backend.NewApp(filepath.Join(dir, "app.db"), filepath.Join("data", "sources"))
	must(err)
	sqlDB, _ := app.DB.DB()
	defer sqlDB.Close()

	server := &http.Server{Addr: "127.0.0.1:18099", Handler: app.Router()}
	go server.ListenAndServe()
	defer server.Close()
	waitReady()

	token := register()

	// 1. Full concept list: wire size with and without gzip, shape check.
	body, wire := fetch("/api/concepts?limit=1000", token, true)
	raw := len(body)
	var rows []map[string]any
	must(json.Unmarshal(body, &rows))
	fmt.Printf("GET /api/concepts          %d rows | %dB raw | %dB gzipped (%.1f%%)\n", len(rows), raw, wire, 100*float64(wire)/float64(raw))
	banned := 0
	for _, row := range rows {
		for _, key := range []string{"unit", "topic", "state"} {
			if _, ok := row[key]; ok {
				banned++
			}
		}
	}
	fmt.Printf("  slim shape: %d rows carry unit/topic/state (want 0); first row has content: %v\n", banned, rows[0]["content"] != nil)

	// 2. Version baseline.
	v1 := version(token)
	fmt.Printf("GET /api/content/version   contentVersion=%d conceptCount=%d stateVersion=%d\n", toInt(v1["contentVersion"]), toInt(v1["conceptCount"]), toInt(v1["stateVersion"]))

	// 3. A mark bumps only stateVersion and costs ~100B.
	markWire := request("PATCH", "/api/concepts/ap-psychology.science-practices.set-a.random-assignment/status", token, `{"status":"fuzzy"}`)
	v2 := version(token)
	fmt.Printf("PATCH /status              %dB | contentVersion %d->%d stateVersion %d->%d (mark must not bump content)\n",
		markWire, toInt(v1["contentVersion"]), toInt(v2["contentVersion"]), toInt(v1["stateVersion"]), toInt(v2["stateVersion"]))

	// 4. Slim states.
	body, wire = fetch("/api/concepts/states", token, true)
	fmt.Printf("GET /api/concepts/states   %dB gzipped\n", wire)
	var states []map[string]any
	must(json.Unmarshal(body, &states))
	fmt.Printf("  non-default states: %d (want 1 after one mark)\n", len(states))

	// 5. Delta at the stored boundary: only boundary-equal rows return (the
	// filter is inclusive by design, so a fresh client never misses a row).
	since := time.UnixMilli(toInt(v2["contentVersion"])).UTC().Format(time.RFC3339Nano)
	body, wire = fetch("/api/concepts?limit=1000&updatedSince="+url.QueryEscape(since), token, true)
	fmt.Printf("delta at boundary          %dB gzipped, rows=%d (want 0-few: only boundary-equal rows)\n", wire, mustLen(body))

	// 6. Admin content edit: delta returns just that row (+ boundary row).
	editWire := request("PATCH", "/api/concepts/ap-psychology.science-practices.set-a.random-assignment/content", token,
		`{"definition":[{"type":"paragraph","text":"perfcheck"}],"examples":[],"pitfalls":[],"notes":[],"source":"perfcheck"}`)
	body, wire = fetch("/api/concepts?limit=1000&updatedSince="+url.QueryEscape(since), token, true)
	fmt.Printf("content edit               %dB request | delta after edit: %dB gzipped, rows=%d (want ~2: edit + boundary)\n", editWire, wire, mustLen(body))

	// 7. Flashcard deck: ids + state only.
	body, wire = fetch("/api/review/next?limit=200", token, true)
	var deck []map[string]any
	must(json.Unmarshal(body, &deck))
	deckFat := 0
	for _, row := range deck {
		if _, ok := row["term"]; ok {
			deckFat++
		}
	}
	fmt.Printf("GET /api/review/next?200   %d cards | %dB gzipped | rows carrying term: %d (want 0)\n", len(deck), wire, deckFat)

	// 8. The tiny version check that replaces a reload's full fetch.
	_, wire = fetch("/api/content/version", token, true)
	fmt.Printf("revalidate cost            %dB gzipped (version check vs %dB full list)\n", wire, raw)
}

func waitReady() {
	for i := 0; i < 100; i++ {
		res, err := http.Get(base + "/api/health")
		if err == nil {
			res.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	must(fmt.Errorf("server never became ready"))
}

func register() string {
	res, err := http.Post(base+"/api/auth/register", "application/json", strings.NewReader(`{"tenantName":"Perf","name":"Perf","email":"perf@example.com","password":"secret"}`))
	must(err)
	defer res.Body.Close()
	var payload struct {
		Token string `json:"token"`
	}
	must(json.NewDecoder(res.Body).Decode(&payload))
	if payload.Token == "" {
		must(fmt.Errorf("no token"))
	}
	return payload.Token
}

func version(token string) map[string]any {
	body, _ := fetch("/api/content/version", token, false)
	var v map[string]any
	must(json.Unmarshal(body, &v))
	return v
}

// fetch returns the decompressed body plus the actual wire byte count.
func fetch(path, token string, gzipped bool) ([]byte, int) {
	req, _ := http.NewRequest("GET", base+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if gzipped {
		req.Header.Set("Accept-Encoding", "gzip")
	}
	res, err := http.DefaultClient.Do(req)
	must(err)
	defer res.Body.Close()
	wire, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		must(fmt.Errorf("%s -> %d %s", path, res.StatusCode, string(wire)))
	}
	if res.Header.Get("Content-Encoding") == "gzip" {
		zr, err := gzip.NewReader(bytes.NewReader(wire))
		must(err)
		body, err := io.ReadAll(zr)
		must(err)
		return body, len(wire)
	}
	return wire, len(wire)
}

func request(method, path, token, jsonBody string) int {
	req, _ := http.NewRequest(method, base+path, strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	must(err)
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		must(fmt.Errorf("%s -> %d %s", path, res.StatusCode, string(body)))
	}
	return len(body)
}

func mustLen(body []byte) int {
	var rows []map[string]any
	must(json.Unmarshal(body, &rows))
	return len(rows)
}

func toInt(v any) int64 {
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return int64(f)
}

func must(err error) {
	if err != nil {
		fmt.Println("perfcheck:", err)
		os.Exit(1)
	}
}
