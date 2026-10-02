package mista

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type reply struct {
	status  int
	body    interface{}
	headers map[string]string
	hangup  bool
}

type recordedCall struct {
	Method  string
	Path    string
	Raw     string
	Query   map[string]string
	Header  http.Header
	Body    map[string]interface{}
	RawBody []byte
}

type recorder struct {
	mu      sync.Mutex
	replies []reply
	calls   []recordedCall
}

func ok(data interface{}) reply {
	return reply{body: map[string]interface{}{"status": "success", "message": nil, "data": data}}
}

func errReply(status int, body interface{}) reply { return reply{status: status, body: body} }

func paginated(rows []map[string]interface{}, current, last int) map[string]interface{} {
	return map[string]interface{}{"current_page": current, "data": rows, "last_page": last, "per_page": 25, "total": len(rows)}
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	raw, _ := io.ReadAll(req.Body)
	c := recordedCall{Method: req.Method, Path: req.URL.Path, Raw: req.URL.RequestURI(), Header: req.Header, Query: map[string]string{}, RawBody: raw}
	for k := range req.URL.Query() {
		c.Query[k] = req.URL.Query().Get(k)
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &c.Body)
	}
	r.calls = append(r.calls, c)
	rep := ok(nil)
	if len(r.replies) > 0 {
		rep, r.replies = r.replies[0], r.replies[1:]
	}
	r.mu.Unlock()

	if rep.hangup {
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		conn.Close()
		return
	}
	for k, v := range rep.headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", "application/json")
	status := rep.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if rep.body != nil {
		_ = json.NewEncoder(w).Encode(rep.body)
	}
}

func newTestClient(t *testing.T, replies ...reply) (*Client, *recorder) {
	t.Helper()
	rec := &recorder{replies: replies}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)
	c := NewClient("test-token", WithBaseURL(srv.URL))
	c.sleep = func(context.Context, time.Duration) error { return nil }
	return c, rec
}

func jsonEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	g, _ := json.Marshal(got)
	w, _ := json.Marshal(want)
	var gv, wv interface{}
	_ = json.Unmarshal(g, &gv)
	_ = json.Unmarshal(w, &wv)
	gj, _ := json.Marshal(gv)
	wj, _ := json.Marshal(wv)
	if string(gj) != string(wj) {
		t.Fatalf("got  %s\nwant %s", gj, wj)
	}
}
