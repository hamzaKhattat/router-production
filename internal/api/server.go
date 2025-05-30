package api

import (
   "encoding/json"
   "fmt"
   "log"
   "net/http"
   "time"
   
   "github.com/gorilla/mux"
   "github.com/router-production/internal/router"
   "github.com/router-production/internal/provider"
)

type Server struct {
   router          *router.Router
   providerManager *provider.Manager
   port            int
}

func NewServer(r *router.Router, pm *provider.Manager, port int) *Server {
   return &Server{
       router:          r,
       providerManager: pm,
       port:            port,
   }
}

func (s *Server) Start() error {
   r := mux.NewRouter()
   
   // Middleware
   r.Use(loggingMiddleware)
   r.Use(corsMiddleware)
   
   // Router endpoints
   r.HandleFunc("/api/processIncoming", s.handleProcessIncoming).Methods("GET", "POST")
   r.HandleFunc("/api/processReturn", s.handleProcessReturn).Methods("GET", "POST")
   r.HandleFunc("/api/stats", s.handleStats).Methods("GET")
   r.HandleFunc("/api/health", s.handleHealth).Methods("GET")
   
   // Provider management endpoints
   r.HandleFunc("/api/providers", s.handleListProviders).Methods("GET")
   r.HandleFunc("/api/providers/{name}/stats", s.handleProviderStats).Methods("GET")
   
   srv := &http.Server{
       Handler:      r,
       Addr:         fmt.Sprintf(":%d", s.port),
       WriteTimeout: 15 * time.Second,
       ReadTimeout:  15 * time.Second,
   }
   
   log.Printf("[API] Server starting on port %d", s.port)
   return srv.ListenAndServe()
}

func loggingMiddleware(next http.Handler) http.Handler {
   return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
       log.Printf("[API] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
       next.ServeHTTP(w, r)
   })
}

func corsMiddleware(next http.Handler) http.Handler {
   return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
       w.Header().Set("Access-Control-Allow-Origin", "*")
       w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
       w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
       
       if r.Method == "OPTIONS" {
           w.WriteHeader(http.StatusOK)
           return
       }
       
       next.ServeHTTP(w, r)
   })
}

func (s *Server) handleProcessIncoming(w http.ResponseWriter, r *http.Request) {
   callID := r.URL.Query().Get("callid")
   ani := r.URL.Query().Get("ani")
   dnis := r.URL.Query().Get("dnis")
   
   if callID == "" || ani == "" || dnis == "" {
       http.Error(w, "Missing parameters", http.StatusBadRequest)
       return
   }
   
   resp, err := s.router.ProcessIncomingCall(callID, ani, dnis)
   if err != nil {
       log.Printf("[API] ProcessIncoming error: %v", err)
       http.Error(w, err.Error(), http.StatusInternalServerError)
       return
   }
   
   w.Header().Set("Content-Type", "application/json")
   json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleProcessReturn(w http.ResponseWriter, r *http.Request) {
   ani2 := r.URL.Query().Get("ani2")
   did := r.URL.Query().Get("did")
   
   if ani2 == "" || did == "" {
       http.Error(w, "Missing parameters", http.StatusBadRequest)
       return
   }
   
   resp, err := s.router.ProcessReturnCall(ani2, did)
   if err != nil {
       log.Printf("[API] ProcessReturn error: %v", err)
       http.Error(w, err.Error(), http.StatusNotFound)
       return
   }
   
   w.Header().Set("Content-Type", "application/json")
   json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
   stats := s.router.GetStatistics()
   
   w.Header().Set("Content-Type", "application/json")
   json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
   w.Header().Set("Content-Type", "application/json")
   json.NewEncoder(w).Encode(map[string]string{
       "status": "ok",
       "time":   time.Now().Format(time.RFC3339),
   })
}

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
   providers := s.providerManager.ListProviders()
   
   w.Header().Set("Content-Type", "application/json")
   json.NewEncoder(w).Encode(providers)
}

func (s *Server) handleProviderStats(w http.ResponseWriter, r *http.Request) {
   vars := mux.Vars(r)
   name := vars["name"]
   
   stats, err := s.providerManager.GetProviderStats(name)
   if err != nil {
       http.Error(w, err.Error(), http.StatusNotFound)
       return
   }
   
   w.Header().Set("Content-Type", "application/json")
   json.NewEncoder(w).Encode(stats)
}
