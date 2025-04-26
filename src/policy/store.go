package policy

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kanywst/zanzibar/src/schema"
)

// Relationship represents a relationship between a resource and a subject
type Relationship struct {
	Resource string `json:"resource"`
	Relation string `json:"relation"`
	Subject  string `json:"subject"`
	// Metadata for consistency
	ZookieToken string    `json:"zookie_token,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Store represents the policy store
type Store struct {
	relationships []Relationship
	schema        *schema.Schema
	mu            sync.RWMutex
	// For consistency tracking
	changeNumber int64
}

// NewStore creates a new policy store
func NewStore(schema *schema.Schema) *Store {
	return &Store{
		relationships: make([]Relationship, 0),
		schema:        schema,
		changeNumber:  1,
	}
}

// AddRelationship adds a new relationship
func (s *Store) AddRelationship(resource, relation, subject string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Parse resource and subject to get types
	resourceParts := strings.SplitN(resource, ":", 2)
	if len(resourceParts) != 2 {
		return "", fmt.Errorf("invalid resource format: %s", resource)
	}
	resourceType := resourceParts[0]

	subjectParts := strings.SplitN(subject, ":", 2)
	if len(subjectParts) != 2 {
		return "", fmt.Errorf("invalid subject format: %s", subject)
	}
	subjectType := subjectParts[0]

	// Validate against schema
	if err := s.schema.ValidateRelationship(resourceType, relation, subjectType); err != nil {
		return "", err
	}

	// Check if relationship already exists
	for _, r := range s.relationships {
		if r.Resource == resource && r.Relation == relation && r.Subject == subject {
			return r.ZookieToken, nil
		}
	}

	// Create zookie token for consistency
	zookieToken := fmt.Sprintf("zk_%d", s.changeNumber)
	s.changeNumber++

	// Add relationship
	s.relationships = append(s.relationships, Relationship{
		Resource:    resource,
		Relation:    relation,
		Subject:     subject,
		ZookieToken: zookieToken,
		UpdatedAt:   time.Now(),
	})

	return zookieToken, nil
}

// RemoveRelationship removes a relationship
func (s *Store) RemoveRelationship(resource, relation, subject string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, r := range s.relationships {
		if r.Resource == resource && r.Relation == relation && r.Subject == subject {
			// Remove by swapping with the last element and truncating
			s.relationships[i] = s.relationships[len(s.relationships)-1]
			s.relationships = s.relationships[:len(s.relationships)-1]
			s.changeNumber++
			return nil
		}
	}

	return fmt.Errorf("relationship not found")
}

// Check checks if a subject has a permission on a resource
func (s *Store) Check(subject, resource, action string) (bool, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Parse resource to get type
	resourceParts := strings.SplitN(resource, ":", 2)
	if len(resourceParts) != 2 {
		return false, "", fmt.Errorf("invalid resource format: %s", resource)
	}
	resourceType := resourceParts[0]

	// Get all relations the subject has with the resource
	relations := s.getRelations(subject, resource)

	// Evaluate permission based on schema
	allowed, err := s.schema.EvaluatePermission(resourceType, action, relations)
	if err != nil {
		return false, "", err
	}

	var reason string
	if allowed {
		reason = fmt.Sprintf("Subject has required relation(s): %v", relations)
	} else {
		reason = fmt.Sprintf("Subject lacks required relation(s) for action: %s", action)
	}

	return allowed, reason, nil
}

// getRelations returns all relations a subject has with a resource
func (s *Store) getRelations(subject, resource string) []string {
	var relations []string

	// Direct relations
	for _, r := range s.relationships {
		if r.Resource == resource && r.Subject == subject {
			relations = append(relations, r.Relation)
		}
	}

	// Get all groups the subject is a member of (directly or indirectly)
	groups := s.getGroupMemberships(subject, make(map[string]bool))

	// Check if any of these groups have relations with the resource
	for groupID := range groups {
		for _, gr := range s.relationships {
			if gr.Resource == resource && gr.Subject == groupID {
				relations = append(relations, gr.Relation)
			}
		}
	}

	return relations
}

// getGroupMemberships recursively finds all groups a subject is a member of
func (s *Store) getGroupMemberships(subject string, visited map[string]bool) map[string]bool {
	groups := make(map[string]bool)

	// Find direct group memberships
	for _, r := range s.relationships {
		if strings.HasPrefix(r.Resource, "group:") && r.Relation == "member" && r.Subject == subject {
			groupID := r.Resource
			if !visited[groupID] {
				groups[groupID] = true
				visited[groupID] = true

				// Recursively find groups that this group is a member of
				nestedGroups := s.getGroupMemberships(groupID, visited)
				for ng := range nestedGroups {
					groups[ng] = true
				}
			}
		}
	}

	return groups
}

// Expand returns all subjects that have a specific relation with a resource
func (s *Store) Expand(resource, relation string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	directSubjects := make(map[string]bool)
	expandedSubjects := make(map[string]bool)

	// Find direct subjects
	for _, r := range s.relationships {
		if r.Resource == resource && r.Relation == relation {
			directSubjects[r.Subject] = true

			// If the subject is a group, expand its members
			if strings.HasPrefix(r.Subject, "group:") {
				s.expandGroupMembers(r.Subject, expandedSubjects, make(map[string]bool))
			}
		}
	}

	// Combine direct subjects and expanded group members
	result := make([]string, 0, len(directSubjects)+len(expandedSubjects))
	for subject := range directSubjects {
		result = append(result, subject)
	}
	for subject := range expandedSubjects {
		result = append(result, subject)
	}

	return result
}

// expandGroupMembers recursively finds all members of a group
func (s *Store) expandGroupMembers(groupID string, result map[string]bool, visited map[string]bool) {
	if visited[groupID] {
		return // Prevent cycles
	}
	visited[groupID] = true

	for _, r := range s.relationships {
		if r.Resource == groupID && r.Relation == "member" {
			result[r.Subject] = true

			// If the member is also a group, recursively expand it
			if strings.HasPrefix(r.Subject, "group:") {
				s.expandGroupMembers(r.Subject, result, visited)
			}
		}
	}
}

// ListRelationships returns all relationships
func (s *Store) ListRelationships() []Relationship {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid race conditions
	relationships := make([]Relationship, len(s.relationships))
	copy(relationships, s.relationships)

	return relationships
}

// GetChangeNumber returns the current change number
func (s *Store) GetChangeNumber() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.changeNumber
}

// InitializeWithSampleData adds sample relationships for testing
func (s *Store) InitializeWithSampleData() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing relationships
	s.relationships = make([]Relationship, 0)

	// Add sample relationships
	s.relationships = append(s.relationships, Relationship{
		Resource:    "document:report",
		Relation:    "owner",
		Subject:     "user:alice",
		ZookieToken: "zk_1",
		UpdatedAt:   time.Now(),
	})

	s.relationships = append(s.relationships, Relationship{
		Resource:    "document:report",
		Relation:    "editor",
		Subject:     "user:bob",
		ZookieToken: "zk_2",
		UpdatedAt:   time.Now(),
	})

	s.relationships = append(s.relationships, Relationship{
		Resource:    "document:report",
		Relation:    "viewer",
		Subject:     "group:engineering",
		ZookieToken: "zk_3",
		UpdatedAt:   time.Now(),
	})

	// Direct group membership
	s.relationships = append(s.relationships, Relationship{
		Resource:    "group:engineering",
		Relation:    "member",
		Subject:     "user:charlie",
		ZookieToken: "zk_4",
		UpdatedAt:   time.Now(),
	})

	// Nested group example
	s.relationships = append(s.relationships, Relationship{
		Resource:    "group:frontend",
		Relation:    "member",
		Subject:     "user:dave",
		ZookieToken: "zk_5",
		UpdatedAt:   time.Now(),
	})

	s.relationships = append(s.relationships, Relationship{
		Resource:    "group:engineering",
		Relation:    "member",
		Subject:     "group:frontend",
		ZookieToken: "zk_6",
		UpdatedAt:   time.Now(),
	})

	s.changeNumber = 7
}
