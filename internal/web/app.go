package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"bubblecopy/internal/model"
	"bubblecopy/internal/session"
)

//go:embed static/index.html
var staticFS embed.FS

type App struct {
	session     *session.Session
	stdout      io.Writer
	stderr      io.Writer
	listen      func(network, address string) (net.Listener, error)
	openBrowser func(url string) error
}

type selectionRequest struct {
	Selected []int `json:"selected"`
}

func Run(tasks []model.Task, workers int, listenAddr string, disableBrowser bool) error {
	app := New(session.New(tasks, workers), nil, nil)
	return app.Run(context.Background(), listenAddr, !disableBrowser)
}

func New(sess *session.Session, stdout io.Writer, stderr io.Writer) *App {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	return &App{
		session:     sess,
		stdout:      stdout,
		stderr:      stderr,
		listen:      net.Listen,
		openBrowser: OpenBrowser,
	}
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("/api/state", a.handleState)
	mux.HandleFunc("/api/selection", a.handleSelection)
	mux.HandleFunc("/api/dry-run", a.handleDryRun)
	mux.HandleFunc("/api/execute", a.handleExecute)
	mux.HandleFunc("/api/reset", a.handleReset)
	return mux
}

func (a *App) Run(ctx context.Context, listenAddr string, shouldOpenBrowser bool) error {
	listener, err := a.listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen web ui: %w", err)
	}

	server := &http.Server{
		Handler: a.Handler(),
	}

	errCh := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	url := listenerURL(listener.Addr())
	fmt.Fprintf(a.stdout, "Web UI: %s\n", url)
	fmt.Fprintln(a.stdout, "Press Ctrl+C to stop.")

	if shouldOpenBrowser {
		if err := a.openBrowser(url); err != nil {
			fmt.Fprintf(a.stderr, "failed to open browser automatically, visit %s manually: %v\n", url, err)
		}
	}

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("serve web ui: %w", err)
		}
		return nil
	case <-sigCtx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown web ui: %w", err)
	}

	if err := <-errCh; err != nil {
		return fmt.Errorf("serve web ui: %w", err)
	}
	return nil
}

func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	content, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "index not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}

func (a *App) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	writeJSON(w, http.StatusOK, a.session.Snapshot())
}

func (a *App) handleSelection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req selectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("decode selection request: %w", err))
		return
	}

	if err := a.session.ReplaceSelection(req.Selected); err != nil {
		writeSessionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, a.session.Snapshot())
}

func (a *App) handleDryRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if err := a.session.DryRun(); err != nil {
		writeSessionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, a.session.Snapshot())
}

func (a *App) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if err := a.session.StartExecution(); err != nil {
		writeSessionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, a.session.Snapshot())
}

func (a *App) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if err := a.session.Reset(); err != nil {
		writeSessionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, a.session.Snapshot())
}

func writeSessionError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, session.ErrBusy), errors.Is(err, session.ErrInvalidPhase):
		status = http.StatusConflict
	case errors.Is(err, session.ErrNoSelection):
		status = http.StatusBadRequest
	}
	writeError(w, status, err)
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func listenerURL(addr net.Addr) string {
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return "http://" + addr.String()
	}

	host := tcpAddr.IP.String()
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if tcpAddr.IP.To4() == nil && host != "" && host[0] != '[' {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("http://%s:%d", host, tcpAddr.Port)
}
