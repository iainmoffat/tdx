package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/ipm/tdx/internal/domain"
)

// SaveFn is a callback to persist the edited template.
type SaveFn func(domain.Template) error

// server holds the state for a single edit session.
type server struct {
	tmpl     domain.Template
	save     SaveFn
	shutdown chan result
}

type result struct {
	saved bool
	err   error
}

// Result is returned to the CLI after the server exits.
type Result struct {
	Saved bool
}

func newServer(tmpl domain.Template, save SaveFn) *server {
	return &server{
		tmpl:     tmpl,
		save:     save,
		shutdown: make(chan result, 1),
	}
}

func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/template", s.handleGetTemplate)
	mux.HandleFunc("/api/save", s.handleSave)
	mux.HandleFunc("/api/cancel", s.handleCancel)
	return mux
}

func (s *server) toResponse() templateResponse {
	resp := templateResponse{Name: s.tmpl.Name}
	for _, r := range s.tmpl.Rows {
		label := r.Label
		if label == "" {
			label = r.Target.DisplayRef
		}
		resp.Rows = append(resp.Rows, templateRowJSON{
			ID:       r.ID,
			Label:    label,
			Group:    r.Target.GroupName,
			TypeName: r.TimeType.Name,
			Hours: hoursJSON{
				Sun: r.Hours.Sun, Mon: r.Hours.Mon, Tue: r.Hours.Tue,
				Wed: r.Hours.Wed, Thu: r.Hours.Thu, Fri: r.Hours.Fri,
				Sat: r.Hours.Sat,
			},
		})
	}
	return resp
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	html, err := injectTemplateData(editorHTML, s.toResponse())
	if err != nil {
		http.Error(w, "failed to render editor", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func (s *server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.toResponse())
}

type saveRequest struct {
	Rows []saveRow `json:"rows"`
}

type saveRow struct {
	ID    string    `json:"id"`
	Hours hoursJSON `json:"hours"`
}

func (s *server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req saveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Build lookup by row ID.
	byID := make(map[string]hoursJSON, len(req.Rows))
	for _, row := range req.Rows {
		byID[row.ID] = row.Hours
	}

	// Apply edits to the template.
	for i, row := range s.tmpl.Rows {
		if h, ok := byID[row.ID]; ok {
			s.tmpl.Rows[i].Hours = domain.WeekHours{
				Sun: h.Sun, Mon: h.Mon, Tue: h.Tue,
				Wed: h.Wed, Thu: h.Thu, Fri: h.Fri,
				Sat: h.Sat,
			}
		}
	}
	s.tmpl.ModifiedAt = time.Now().UTC()

	// Save via callback.
	if s.save != nil {
		if err := s.save(s.tmpl); err != nil {
			http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	select {
	case s.shutdown <- result{saved: true}:
	default:
	}
}

func (s *server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	select {
	case s.shutdown <- result{saved: false}:
	default:
	}
}

// Run starts the HTTP server, opens the browser, and blocks until save
// or cancel. Returns whether the template was saved.
func Run(tmpl domain.Template, save SaveFn) (Result, error) {
	srv := newServer(tmpl, save)

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return Result{}, fmt.Errorf("listen: %w", err)
	}

	addr := listener.Addr().String()
	url := "http://" + addr

	httpSrv := &http.Server{Handler: srv.handler()}
	go func() { _ = httpSrv.Serve(listener) }()

	// Open browser (best effort).
	if err := openBrowser(url); err != nil {
		_, _ = fmt.Printf("Could not open browser: %v\nOpen %s manually.\n", err, url)
	}

	// Wait for save or cancel.
	res := <-srv.shutdown

	// Graceful shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)

	return Result{Saved: res.saved}, res.err
}

// openBrowser opens the given URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
