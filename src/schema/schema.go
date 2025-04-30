package schema

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Definition represents a resource type definition
type Definition struct {
	Type        string                `json:"type"`
	Relations   map[string]Relation   `json:"relations"`
	Permissions map[string]Permission `json:"permissions"`
}

// Relation defines a relationship between resources
type Relation struct {
	Subjects       []Subject       `json:"subjects"`
	UsersetRewrite *UsersetRewrite `json:"userset_rewrite,omitempty"`
}

// Subject defines a subject that can be in a relation
type Subject struct {
	Type     string `json:"type"`
	Relation string `json:"relation,omitempty"`
}

// Permission defines a permission expression
type Permission struct {
	Expression string `json:"expression"`
}

// Schema represents the complete schema with all definitions
type Schema struct {
	Definitions map[string]*Definition `json:"definitions"`
	mu          sync.RWMutex
}

// NewSchema creates a new empty schema
func NewSchema() *Schema {
	return &Schema{
		Definitions: make(map[string]*Definition),
	}
}

// AddDefinition adds a new resource type definition to the schema
func (s *Schema) AddDefinition(def *Definition) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Definitions[def.Type]; exists {
		return fmt.Errorf("definition for type %s already exists", def.Type)
	}

	s.Definitions[def.Type] = def
	return nil
}

// GetDefinition returns a definition by type
func (s *Schema) GetDefinition(typeName string) (*Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	def, exists := s.Definitions[typeName]
	if !exists {
		return nil, fmt.Errorf("definition for type %s not found", typeName)
	}

	return def, nil
}

// LoadDefaultSchema loads a default schema with common resource types
func LoadDefaultSchema() *Schema {
	schema := NewSchema()

	// User definition
	userDef := &Definition{
		Type:      "user",
		Relations: make(map[string]Relation),
	}
	schema.AddDefinition(userDef)

	// Document definition
	docDef := &Definition{
		Type: "document",
		Relations: map[string]Relation{
			"owner": {
				Subjects: []Subject{
					{Type: "user"},
				},
			},
			"editor": {
				Subjects: []Subject{
					{Type: "user"},
				},
			},
			"viewer": {
				Subjects: []Subject{
					{Type: "user"},
					{Type: "group", Relation: "member"},
				},
			},
			"parent": {
				Subjects: []Subject{
					{Type: "folder"},
				},
			},
		},
		Permissions: map[string]Permission{
			"view": {
				Expression: "owner | editor | viewer",
			},
			"edit": {
				Expression: "owner | editor",
			},
			"delete": {
				Expression: "owner",
			},
		},
	}
	schema.AddDefinition(docDef)

	// Folder definition
	folderDef := &Definition{
		Type: "folder",
		Relations: map[string]Relation{
			"owner": {
				Subjects: []Subject{
					{Type: "user"},
				},
			},
			"editor": {
				Subjects: []Subject{
					{Type: "user"},
				},
			},
			"viewer": {
				Subjects: []Subject{
					{Type: "user"},
					{Type: "group", Relation: "member"},
				},
			},
		},
		Permissions: map[string]Permission{
			"view": {
				Expression: "owner | editor | viewer",
			},
			"edit": {
				Expression: "owner | editor",
			},
			"delete": {
				Expression: "owner",
			},
		},
	}
	schema.AddDefinition(folderDef)

	// Group definition
	groupDef := &Definition{
		Type: "group",
		Relations: map[string]Relation{
			"member": {
				Subjects: []Subject{
					{Type: "user"},
					{Type: "group"}, // Allow groups to be members of other groups for nested groups
				},
			},
		},
	}
	schema.AddDefinition(groupDef)

	return schema
}

// ValidateRelationship validates if a relationship is allowed by the schema
func (s *Schema) ValidateRelationship(resourceType, relation, subjectType string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	def, exists := s.Definitions[resourceType]
	if !exists {
		return fmt.Errorf("resource type %s not defined in schema", resourceType)
	}

	rel, exists := def.Relations[relation]
	if !exists {
		return fmt.Errorf("relation %s not defined for resource type %s", relation, resourceType)
	}

	for _, subject := range rel.Subjects {
		if subject.Type == subjectType {
			return nil
		}
	}

	return fmt.Errorf("subject type %s not allowed in relation %s for resource type %s", subjectType, relation, resourceType)
}

// EvaluatePermission evaluates if a permission is granted based on relations
func (s *Schema) EvaluatePermission(resourceType, permission string, relations []string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	def, exists := s.Definitions[resourceType]
	if !exists {
		return false, fmt.Errorf("resource type %s not defined in schema", resourceType)
	}

	perm, exists := def.Permissions[permission]
	if !exists {
		return false, fmt.Errorf("permission %s not defined for resource type %s", permission, resourceType)
	}

	// Simple expression evaluation
	// Format: "relation1 | relation2 | relation3"
	expr := perm.Expression
	parts := strings.Split(expr, "|")

	for _, part := range parts {
		relation := strings.TrimSpace(part)
		for _, r := range relations {
			if r == relation {
				return true, nil
			}
		}
	}

	return false, nil
}

// ToJSON converts the schema to JSON
func (s *Schema) ToJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return json.Marshal(s.Definitions)
}

// FromJSON loads the schema from JSON
func (s *Schema) FromJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return json.Unmarshal(data, &s.Definitions)
}
