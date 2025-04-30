package schema

import (
	"encoding/json"
	"fmt"
)

// UsersetRewriteType defines the type of userset rewrite rule
type UsersetRewriteType string

const (
	// This represents direct relationships from stored relation tuples
	UsersetRewriteThis UsersetRewriteType = "this"
	// ComputedUserset represents a userset computed from another relation on the same object
	UsersetRewriteComputedUserset UsersetRewriteType = "computed_userset"
	// TupleToUserset represents a userset computed from a relation on another object
	UsersetRewriteTupleToUserset UsersetRewriteType = "tuple_to_userset"
	// Union represents a union of multiple usersets
	UsersetRewriteUnion UsersetRewriteType = "union"
	// Intersection represents an intersection of multiple usersets
	UsersetRewriteIntersection UsersetRewriteType = "intersection"
	// Exclusion represents a set difference between two usersets
	UsersetRewriteExclusion UsersetRewriteType = "exclusion"
)

// UsersetRewrite defines a rule for computing a userset
type UsersetRewrite struct {
	Type            UsersetRewriteType `json:"type"`
	ComputedUserset *ComputedUserset   `json:"computed_userset,omitempty"`
	TupleToUserset  *TupleToUserset    `json:"tuple_to_userset,omitempty"`
	Children        []*UsersetRewrite  `json:"children,omitempty"`
}

// ComputedUserset represents a userset computed from another relation on the same object
type ComputedUserset struct {
	Relation string `json:"relation"`
}

// TupleToUserset represents a userset computed from a relation on another object
type TupleToUserset struct {
	Tupleset        Tupleset        `json:"tupleset"`
	ComputedUserset ComputedUserset `json:"computed_userset"`
}

// Tupleset defines a set of tuples to look up
type Tupleset struct {
	Relation string `json:"relation"`
}

// NewThisRewrite creates a new "this" userset rewrite rule
func NewThisRewrite() *UsersetRewrite {
	return &UsersetRewrite{
		Type: UsersetRewriteThis,
	}
}

// NewComputedUsersetRewrite creates a new computed_userset rewrite rule
func NewComputedUsersetRewrite(relation string) *UsersetRewrite {
	return &UsersetRewrite{
		Type: UsersetRewriteComputedUserset,
		ComputedUserset: &ComputedUserset{
			Relation: relation,
		},
	}
}

// NewTupleToUsersetRewrite creates a new tuple_to_userset rewrite rule
func NewTupleToUsersetRewrite(tupleRelation, computedRelation string) *UsersetRewrite {
	return &UsersetRewrite{
		Type: UsersetRewriteTupleToUserset,
		TupleToUserset: &TupleToUserset{
			Tupleset: Tupleset{
				Relation: tupleRelation,
			},
			ComputedUserset: ComputedUserset{
				Relation: computedRelation,
			},
		},
	}
}

// NewUnionRewrite creates a new union rewrite rule
func NewUnionRewrite(children ...*UsersetRewrite) *UsersetRewrite {
	return &UsersetRewrite{
		Type:     UsersetRewriteUnion,
		Children: children,
	}
}

// NewIntersectionRewrite creates a new intersection rewrite rule
func NewIntersectionRewrite(children ...*UsersetRewrite) *UsersetRewrite {
	return &UsersetRewrite{
		Type:     UsersetRewriteIntersection,
		Children: children,
	}
}

// NewExclusionRewrite creates a new exclusion rewrite rule
func NewExclusionRewrite(include, exclude *UsersetRewrite) *UsersetRewrite {
	return &UsersetRewrite{
		Type:     UsersetRewriteExclusion,
		Children: []*UsersetRewrite{include, exclude},
	}
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (u *UsersetRewrite) UnmarshalJSON(data []byte) error {
	// First, unmarshal into a map to determine the type
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Determine the type
	if _, ok := raw["_this"]; ok {
		u.Type = UsersetRewriteThis
		return nil
	}

	if computedUsersetData, ok := raw["computed_userset"]; ok {
		u.Type = UsersetRewriteComputedUserset
		var cu ComputedUserset
		if err := json.Unmarshal(computedUsersetData, &cu); err != nil {
			return err
		}
		u.ComputedUserset = &cu
		return nil
	}

	if tupleToUsersetData, ok := raw["tuple_to_userset"]; ok {
		u.Type = UsersetRewriteTupleToUserset
		var ttu TupleToUserset
		if err := json.Unmarshal(tupleToUsersetData, &ttu); err != nil {
			return err
		}
		u.TupleToUserset = &ttu
		return nil
	}

	if unionData, ok := raw["union"]; ok {
		u.Type = UsersetRewriteUnion
		var unionMap map[string]json.RawMessage
		if err := json.Unmarshal(unionData, &unionMap); err != nil {
			return err
		}
		childrenData, ok := unionMap["child"]
		if !ok {
			return fmt.Errorf("union rewrite rule must have 'child' field")
		}
		var children []*UsersetRewrite
		if err := json.Unmarshal(childrenData, &children); err != nil {
			return err
		}
		u.Children = children
		return nil
	}

	if intersectionData, ok := raw["intersection"]; ok {
		u.Type = UsersetRewriteIntersection
		var intersectionMap map[string]json.RawMessage
		if err := json.Unmarshal(intersectionData, &intersectionMap); err != nil {
			return err
		}
		childrenData, ok := intersectionMap["child"]
		if !ok {
			return fmt.Errorf("intersection rewrite rule must have 'child' field")
		}
		var children []*UsersetRewrite
		if err := json.Unmarshal(childrenData, &children); err != nil {
			return err
		}
		u.Children = children
		return nil
	}

	if exclusionData, ok := raw["exclusion"]; ok {
		u.Type = UsersetRewriteExclusion
		var exclusionMap map[string]json.RawMessage
		if err := json.Unmarshal(exclusionData, &exclusionMap); err != nil {
			return err
		}
		baseData, ok := exclusionMap["base"]
		if !ok {
			return fmt.Errorf("exclusion rewrite rule must have 'base' field")
		}
		var base UsersetRewrite
		if err := json.Unmarshal(baseData, &base); err != nil {
			return err
		}
		subtractData, ok := exclusionMap["subtract"]
		if !ok {
			return fmt.Errorf("exclusion rewrite rule must have 'subtract' field")
		}
		var subtract UsersetRewrite
		if err := json.Unmarshal(subtractData, &subtract); err != nil {
			return err
		}
		u.Children = []*UsersetRewrite{&base, &subtract}
		return nil
	}

	return fmt.Errorf("unknown userset rewrite type")
}

// MarshalJSON implements the json.Marshaler interface
func (u *UsersetRewrite) MarshalJSON() ([]byte, error) {
	switch u.Type {
	case UsersetRewriteThis:
		return json.Marshal(map[string]interface{}{
			"_this": struct{}{},
		})
	case UsersetRewriteComputedUserset:
		return json.Marshal(map[string]interface{}{
			"computed_userset": u.ComputedUserset,
		})
	case UsersetRewriteTupleToUserset:
		return json.Marshal(map[string]interface{}{
			"tuple_to_userset": u.TupleToUserset,
		})
	case UsersetRewriteUnion:
		return json.Marshal(map[string]interface{}{
			"union": map[string]interface{}{
				"child": u.Children,
			},
		})
	case UsersetRewriteIntersection:
		return json.Marshal(map[string]interface{}{
			"intersection": map[string]interface{}{
				"child": u.Children,
			},
		})
	case UsersetRewriteExclusion:
		if len(u.Children) != 2 {
			return nil, fmt.Errorf("exclusion rewrite rule must have exactly 2 children")
		}
		return json.Marshal(map[string]interface{}{
			"exclusion": map[string]interface{}{
				"base":     u.Children[0],
				"subtract": u.Children[1],
			},
		})
	default:
		return nil, fmt.Errorf("unknown userset rewrite type: %s", u.Type)
	}
}

// UpdateDefinitionWithUsersetRewrites updates the schema definition to include userset rewrites
func (s *Schema) UpdateDefinitionWithUsersetRewrites() error {
	// Update the document definition to include userset rewrites
	docDef, err := s.GetDefinition("document")
	if err != nil {
		return err
	}

	// Update the viewer relation to include userset rewrites
	viewerRelation := docDef.Relations["viewer"]

	// Create a userset rewrite rule for the viewer relation:
	// viewer = this | editor | parent#viewer
	viewerRewrite := NewUnionRewrite(
		NewThisRewrite(),
		NewComputedUsersetRewrite("editor"),
		NewTupleToUsersetRewrite("parent", "viewer"),
	)

	// Update the relation with the rewrite rule
	viewerRelation.UsersetRewrite = viewerRewrite
	docDef.Relations["viewer"] = viewerRelation

	// Update the editor relation to include userset rewrites
	editorRelation := docDef.Relations["editor"]

	// Create a userset rewrite rule for the editor relation:
	// editor = this | owner
	editorRewrite := NewUnionRewrite(
		NewThisRewrite(),
		NewComputedUsersetRewrite("owner"),
	)

	// Update the relation with the rewrite rule
	editorRelation.UsersetRewrite = editorRewrite
	docDef.Relations["editor"] = editorRelation

	return nil
}
