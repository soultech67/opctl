package api

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // registers /debug/pprof handlers on http.DefaultServeMux; only reachable when explicitly routed below
	"os"
	"runtime/debug"
	"strings"

	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	core "github.com/opctl/opctl/sdks/go/node"
	"github.com/opctl/opctl/sdks/go/node/api/handler"
	"github.com/opctl/opctl/webapp"
)

// debugPprofEnvVar gates the localhost-only /debug/pprof profiling endpoints.
const debugPprofEnvVar = "OPCTL_DEBUG_PPROF"

func pprofEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(debugPprofEnvVar))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

/*
*
Listen for API requests
*/
func Listen(
	ctx context.Context,
	address string,
	opctlNodeCore core.Core,
) error {
	router := mux.NewRouter()
	router.UseEncodedPath()

	router.PathPrefix("/api/").Handler(
		stripPrefixAndRecover(
			"/api/",
			handler.New(
				opctlNodeCore,
			),
		),
	)

	if pprofEnabled() {
		// Profiling endpoints for diagnosing a wedged/leaking daemon. The API
		// server binds to APIListenAddress (127.0.0.1 by default), so these are
		// localhost-only; additionally gated behind OPCTL_DEBUG_PPROF (off by
		// default). net/http/pprof registers its handlers on the default mux.
		router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
		slog.Info("pprof debug endpoints enabled", "pathPrefix", "/debug/pprof/", "address", address)
	}

	buildFS, err := fs.Sub(webapp.Build, "build")
	if err != nil {
		return err
	}

	router.PathPrefix("/").Handler(http.FileServer(http.FS(buildFS)))

	apiServer := http.Server{
		Addr:    address,
		Handler: handlers.CORS()(router),
	}

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// little hammer
		apiServer.Shutdown(ctx)

		// big hammer
		apiServer.Close()
	}()

	if err := apiServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}

// StripPrefix returns a handler that serves HTTP requests
// by removing the given prefix from the request URL's Path
// and invoking the handler h. StripPrefix handles a
// request for a path that doesn't begin with prefix by
// replying with an HTTP 404 not found error.
func stripPrefixAndRecover(prefix string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// recover per-request so a panic in a handler is logged durably and
		// doesn't take down the long-lived daemon. (Previously the recover sat
		// in this constructor and only covered setup, not request handling.)
		defer func() {
			if panic := recover(); panic != nil {
				slog.Error("recovered from panic in api handler",
					"path", r.URL.Path, "panic", panic, "stack", string(debug.Stack()))
				// best-effort: surface a 500 so callers see a failure rather than an
				// empty/200 response. If the handler already wrote a header this is a
				// no-op (net/http logs a superfluous-WriteHeader warning, harmless).
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		if prefix == "" {
			h.ServeHTTP(w, r)
			return
		}

		if p := strings.TrimPrefix(r.URL.Path, prefix); len(p) < len(r.URL.Path) {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
			r.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, prefix)
			h.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
	})
}
