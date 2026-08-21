package gateway

import (
	"encoding/json"
	"net/http"
)

// NewMux builds the gateway's HTTP router.
func NewMux(cfg Config, proxy *Proxy) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /graphs", func(w http.ResponseWriter, r *http.Request) {
		graphs := cfg.Graphs
		if graphs == nil {
			graphs = []string{} // avoid encoding a nil slice as JSON null
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]string{"graphs": graphs})
	})

	mux.HandleFunc("POST /graphs/{graph}/v1/invoke", func(w http.ResponseWriter, r *http.Request) {
		graph := r.PathValue("graph")
		if !cfg.Allowed(graph) {
			http.NotFound(w, r)
			return
		}
		proxy.ServeUpstream(w, r, graph, "/v1/invoke")
	})

	mux.HandleFunc("GET /graphs/{graph}/v1/events", func(w http.ResponseWriter, r *http.Request) {
		graph := r.PathValue("graph")
		if !cfg.Allowed(graph) {
			http.NotFound(w, r)
			return
		}
		proxy.ServeUpstream(w, r, graph, "/v1/events")
	})

	return mux
}
