package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/kanywst/zanzibar/src/policy"
	"github.com/kanywst/zanzibar/src/schema"
)

// Server represents the API server
type Server struct {
	policyStore *policy.Store
	schema      *schema.Schema
}

// NewServer creates a new API server
func NewServer(policyStore *policy.Store, schema *schema.Schema) *Server {
	return &Server{
		policyStore: policyStore,
		schema:      schema,
	}
}

// Principal represents a principal in an authorization request
type Principal struct {
	ID         string                 `json:"id"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// Resource represents a resource in an authorization request
type Resource struct {
	ID         string                 `json:"id"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// AuthorizeRequest represents an authorization request
type AuthorizeRequest struct {
	Principal Principal              `json:"principal"`
	Resource  Resource               `json:"resource"`
	Action    string                 `json:"action"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// AuthorizeResponse represents an authorization response
type AuthorizeResponse struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

// RelationshipRequest represents a relationship management request
type RelationshipRequest struct {
	Resource Resource  `json:"resource"`
	Relation string    `json:"relation"`
	Subject  Principal `json:"subject"`
}

// RelationshipResponse represents a relationship management response
type RelationshipResponse struct {
	ZookieToken string `json:"zookie_token"`
}

// Start starts the API server
func (s *Server) Start(port int) error {
	// Register handlers
	http.HandleFunc("/v1/authorize", s.handleAuthorize)
	http.HandleFunc("/v1/relationships", s.handleRelationships)
	http.HandleFunc("/v1/resources/", s.handleResources)
	http.HandleFunc("/v1/schema", s.handleSchema)
	http.HandleFunc("/health", s.handleHealth)

	// Start server
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// handleAuthorize handles authorization requests
func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Principal.ID == "" || req.Resource.ID == "" || req.Action == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Check authorization
	allowed, reason, err := s.policyStore.Check(req.Principal.ID, req.Resource.ID, req.Action)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Prepare response
	decision := "DENY"
	if allowed {
		decision = "ALLOW"
	}

	resp := AuthorizeResponse{
		Decision: decision,
		Reason:   reason,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleRelationships handles relationship management
func (s *Server) handleRelationships(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.addRelationship(w, r)
	case http.MethodDelete:
		s.removeRelationship(w, r)
	case http.MethodGet:
		s.listRelationships(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// addRelationship adds a new relationship
func (s *Server) addRelationship(w http.ResponseWriter, r *http.Request) {
	var req RelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Resource.ID == "" || req.Relation == "" || req.Subject.ID == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Add relationship
	zookieToken, err := s.policyStore.AddRelationship(req.Resource.ID, req.Relation, req.Subject.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Prepare response
	resp := RelationshipResponse{
		ZookieToken: zookieToken,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// removeRelationship removes a relationship
func (s *Server) removeRelationship(w http.ResponseWriter, r *http.Request) {
	var req RelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Resource.ID == "" || req.Relation == "" || req.Subject.ID == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Remove relationship
	err := s.policyStore.RemoveRelationship(req.Resource.ID, req.Relation, req.Subject.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Send response
	w.WriteHeader(http.StatusNoContent)
}

// listRelationships lists all relationships
func (s *Server) listRelationships(w http.ResponseWriter, r *http.Request) {
	relationships := s.policyStore.ListRelationships()

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(relationships)
}

// handleResources handles resource-related operations
func (s *Server) handleResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse path: /v1/resources/{resource_id}/relations/{relation}/subjects
	path := strings.TrimPrefix(r.URL.Path, "/v1/resources/")
	parts := strings.Split(path, "/")

	if len(parts) != 3 || parts[1] != "relations" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	resourceID := parts[0]
	relation := parts[2]

	// Get subjects
	subjects := s.policyStore.Expand(resourceID, relation)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"subjects": subjects})
}

// handleSchema handles schema operations
func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Get schema
		schemaJSON, err := s.schema.ToJSON()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Send response
		w.Header().Set("Content-Type", "application/json")
		w.Write(schemaJSON)

	case http.MethodPut:
		// Update schema
		var newSchema schema.Schema
		if err := json.NewDecoder(r.Body).Decode(&newSchema); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Replace schema (simplified for demo)
		s.schema = &newSchema

		// Send response
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleHealth handles health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
