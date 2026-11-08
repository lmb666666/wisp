package http

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lmb666666/wisp/internal/webhook"
)

func initTestOwner(t *testing.T, base string, owner *http.Client) {
	t.Helper()
	code, body := ownerJSON(owner, "POST", base+"/api/v1/auth/init",
		map[string]string{"username": "liang", "password": "password123"})
	if code != http.StatusCreated {
		t.Fatalf("init: %d %s", code, body)
	}
}

func TestMoodsAPI(t *testing.T) {
	base, owner, guest := newTestServer(t)
	initTestOwner(t, base, owner)
	today := time.Now().UTC().Format("2006-01-02")
	moodsURL := base + "/api/v1/moods"

	// 游客可读、不可写
	code, body := ownerJSON(guest, "GET", moodsURL+"?from="+today+"&to="+today, nil)
	if code != http.StatusOK || !strings.Contains(string(body), `"items":[]`) {
		t.Fatalf("guest list = %d %s", code, body)
	}
	if code, _ := ownerJSON(guest, "PUT", moodsURL+"/"+today, map[string]string{"mood": "calm"}); code != http.StatusUnauthorized {
		t.Fatalf("guest set = %d, want 401", code)
	}
	if code, _ := ownerJSON(guest, "GET", moodsURL+"/latest", nil); code != http.StatusNotFound {
		t.Fatalf("empty latest = %d, want 404", code)
	}

	// 校验
	for _, tc := range []struct {
		name string
		date string
		mood string
	}{
		{"bad mood", today, "great"},
		{"bad date", "2026-02-30", "calm"},
		{"too old", "1999-12-31", "calm"},
		{"future", time.Now().UTC().Add(48 * time.Hour).Format("2006-01-02"), "calm"},
	} {
		if code, _ := ownerJSON(owner, "PUT", moodsURL+"/"+tc.date, map[string]string{"mood": tc.mood}); code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", tc.name, code)
		}
	}
	if code, _ := ownerJSON(owner, "PUT", moodsURL+"/"+today, map[string]string{}); code != http.StatusBadRequest {
		t.Errorf("missing mood = %d, want 400", code)
	}
	if code, _ := ownerJSON(owner, "DELETE", moodsURL+"/not-a-date", nil); code != http.StatusBadRequest {
		t.Errorf("delete bad date = %d, want 400", code)
	}
	if code, _ := ownerJSON(owner, "GET", moodsURL+"?from="+today, nil); code != http.StatusBadRequest {
		t.Errorf("missing to = %d, want 400", code)
	}
	if code, _ := ownerJSON(owner, "GET", moodsURL+"?from="+today+"&to=2000-01-01", nil); code != http.StatusBadRequest {
		t.Errorf("from > to = %d, want 400", code)
	}
	if code, _ := ownerJSON(owner, "GET", moodsURL+"?from=2000-01-01&to=2026-01-01", nil); code != http.StatusBadRequest {
		t.Errorf("range > 10y = %d, want 400", code)
	}

	// 写入 + 同日覆盖
	if code, body = ownerJSON(owner, "PUT", moodsURL+"/"+today, map[string]string{"mood": "calm"}); code != http.StatusOK {
		t.Fatalf("set = %d %s", code, body)
	}
	if code, _ = ownerJSON(owner, "PUT", moodsURL+"/"+today, map[string]string{"mood": "happy"}); code != http.StatusOK {
		t.Fatalf("overwrite = %d", code)
	}
	code, body = ownerJSON(guest, "GET", moodsURL+"?from="+today+"&to="+today, nil)
	if code != http.StatusOK || !strings.Contains(string(body), `"date":"`+today+`"`) || !strings.Contains(string(body), `"mood":"happy"`) {
		t.Fatalf("list after upsert = %d %s", code, body)
	}
	if strings.Count(string(body), `"date":`) != 1 {
		t.Fatalf("same day should keep one row: %s", body)
	}

	// 补记历史日期 + latest 取最大值
	past := "2020-01-02"
	if code, _ = ownerJSON(owner, "PUT", moodsURL+"/"+past, map[string]string{"mood": "tired"}); code != http.StatusOK {
		t.Fatalf("backfill = %d", code)
	}
	if code, body = ownerJSON(guest, "GET", moodsURL+"/latest", nil); code != http.StatusOK ||
		!strings.Contains(string(body), `"date":"`+today+`"`) {
		t.Fatalf("latest = %d %s", code, body)
	}
	code, body = ownerJSON(guest, "GET", moodsURL+"?from="+past+"&to="+today, nil)
	if code != http.StatusOK || strings.Count(string(body), `"date":`) != 2 {
		t.Fatalf("range = %d %s", code, body)
	}

	// read PAT 可读不可写；write PAT 可写
	_, body = ownerJSON(owner, "POST", base+"/api/v1/auth/tokens", map[string]string{"name": "reader", "scopes": "read"})
	var readTok struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(body, &readTok)
	readC := newClient()
	readC.Transport = bearerTransport{token: readTok.Token, base: http.DefaultTransport}
	if code, _ = ownerJSON(readC, "PUT", moodsURL+"/"+today, map[string]string{"mood": "low"}); code != http.StatusForbidden {
		t.Errorf("read PAT set = %d, want 403", code)
	}

	_, body = ownerJSON(owner, "POST", base+"/api/v1/auth/tokens", map[string]string{"name": "writer", "scopes": "read,write"})
	var writeTok struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(body, &writeTok)
	writeC := newClient()
	writeC.Transport = bearerTransport{token: writeTok.Token, base: http.DefaultTransport}
	if code, _ = ownerJSON(writeC, "PUT", moodsURL+"/"+today, map[string]string{"mood": "low"}); code != http.StatusOK {
		t.Fatalf("write PAT set = %d", code)
	}

	// 删除幂等
	if code, _ = ownerJSON(owner, "DELETE", moodsURL+"/"+today, nil); code != http.StatusNoContent {
		t.Fatalf("delete = %d", code)
	}
	if code, _ = ownerJSON(owner, "DELETE", moodsURL+"/"+today, nil); code != http.StatusNoContent {
		t.Fatalf("delete again = %d", code)
	}
	code, body = ownerJSON(guest, "GET", moodsURL+"?from="+today+"&to="+today, nil)
	if code != http.StatusOK || !strings.Contains(string(body), `"items":[]`) {
		t.Fatalf("after delete = %d %s", code, body)
	}
}

func TestMoodWebhookEvents(t *testing.T) {
	base, owner, _ := newTestServer(t)
	initTestOwner(t, base, owner)

	type delivery struct {
		event string
		body  []byte
		sig   string
	}
	ch := make(chan delivery, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ch <- delivery{event: r.Header.Get("X-Wisp-Event"), body: body, sig: r.Header.Get("X-Wisp-Signature")}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	code, body := ownerJSON(owner, "POST", base+"/api/v1/admin/webhooks",
		map[string]string{"url": srv.URL, "events": "mood.updated,mood.deleted"})
	if code != http.StatusCreated {
		t.Fatalf("create webhook = %d %s", code, body)
	}
	var hook struct {
		Secret string `json:"secret"`
	}
	_ = json.Unmarshal(body, &hook)

	date := time.Now().UTC().Format("2006-01-02")
	if code, _ = ownerJSON(owner, "PUT", base+"/api/v1/moods/"+date, map[string]string{"mood": "calm"}); code != http.StatusOK {
		t.Fatalf("set mood = %d", code)
	}
	if code, _ = ownerJSON(owner, "DELETE", base+"/api/v1/moods/"+date, nil); code != http.StatusNoContent {
		t.Fatalf("delete mood = %d", code)
	}

	got := map[string]delivery{}
	deadline := time.After(3 * time.Second)
	for len(got) < 2 {
		select {
		case d := <-ch:
			got[d.event] = d
		case <-deadline:
			t.Fatalf("webhook events = %v", got)
		}
	}
	for _, event := range []string{"mood.updated", "mood.deleted"} {
		d, ok := got[event]
		if !ok {
			t.Fatalf("missing %s: %v", event, got)
		}
		if want := "sha256=" + webhook.Sign(hook.Secret, d.body); d.sig != want {
			t.Errorf("%s signature = %q, want %q", event, d.sig, want)
		}
	}

	var updated struct {
		Data struct {
			Date string `json:"date"`
			Mood string `json:"mood"`
		} `json:"data"`
	}
	if err := json.Unmarshal(got["mood.updated"].body, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Data.Date != date || updated.Data.Mood != "calm" {
		t.Errorf("mood.updated payload = %+v", updated.Data)
	}
	var deleted struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(got["mood.deleted"].body, &deleted); err != nil {
		t.Fatal(err)
	}
	if deleted.Data["date"] != date || deleted.Data["mood"] != "" {
		t.Errorf("mood.deleted payload = %+v", deleted.Data)
	}
}

func TestMoodOnlyCapsuleImport(t *testing.T) {
	base, owner, guest := newTestServer(t)
	initTestOwner(t, base, owner)

	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	if w, err := zw.Create("wisp.yaml"); err != nil {
		t.Fatal(err)
	} else if _, err := w.Write([]byte("version: 1\nexported_at: 1790000000\n")); err != nil {
		t.Fatal(err)
	}
	if w, err := zw.Create("moods.yaml"); err != nil {
		t.Fatal(err)
	} else if _, err := w.Write([]byte("- date: \"2020-01-02\"\n  mood: calm\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("source", "wisp"); err != nil {
		t.Fatal(err)
	}
	fw, err := mw.CreateFormFile("file", "moods.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(archive.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/admin/import", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := owner.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(raw), `"moods_imported":1`) {
		t.Fatalf("mood-only import = %d %s", resp.StatusCode, raw)
	}

	code, got := ownerJSON(guest, "GET", base+"/api/v1/moods?from=2020-01-02&to=2020-01-02", nil)
	if code != http.StatusOK || !strings.Contains(string(got), `"mood":"calm"`) {
		t.Fatalf("imported mood = %d %s", code, got)
	}
}
