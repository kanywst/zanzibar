package policy

import (
	"fmt"
	"strings"

	"github.com/kanywst/zanzibar/src/schema"
)

// Evaluator handles the evaluation of userset rewrite rules
type Evaluator struct {
	store *Store
}

// NewEvaluator creates a new evaluator
func NewEvaluator(store *Store) *Evaluator {
	return &Evaluator{
		store: store,
	}
}

// EvaluateUserset evaluates a userset rewrite rule for a given object and relation
func (e *Evaluator) EvaluateUserset(objectID, relation, subject string) (bool, error) {
	// Parse resource to get type
	resourceParts := strings.SplitN(objectID, ":", 2)
	if len(resourceParts) != 2 {
		return false, fmt.Errorf("invalid resource format: %s", objectID)
	}
	resourceType := resourceParts[0]

	// Get the definition for the resource type
	def, err := e.store.schema.GetDefinition(resourceType)
	if err != nil {
		return false, err
	}

	// Get the relation definition
	rel, exists := def.Relations[relation]
	if !exists {
		return false, fmt.Errorf("relation %s not defined for resource type %s", relation, resourceType)
	}

	// If there's no userset rewrite rule, fall back to direct relation check
	if rel.UsersetRewrite == nil {
		// Check direct relation
		for _, r := range e.store.relationships {
			if r.Resource == objectID && r.Relation == relation && r.Subject == subject {
				return true, nil
			}
		}

		// Check group membership
		if strings.HasPrefix(subject, "user:") {
			groups := e.store.getGroupMemberships(subject, make(map[string]bool))
			for groupID := range groups {
				for _, r := range e.store.relationships {
					if r.Resource == objectID && r.Relation == relation && r.Subject == groupID {
						return true, nil
					}
				}
			}
		}

		return false, nil
	}

	// Evaluate the userset rewrite rule
	return e.evaluateUsersetRewrite(objectID, rel.UsersetRewrite, subject)
}

// evaluateUsersetRewrite evaluates a userset rewrite rule
func (e *Evaluator) evaluateUsersetRewrite(objectID string, rewrite *schema.UsersetRewrite, subject string) (bool, error) {
	switch rewrite.Type {
	case schema.UsersetRewriteThis:
		// Check direct relation (this)
		for _, r := range e.store.relationships {
			if r.Resource == objectID && r.Subject == subject {
				return true, nil
			}
		}
		return false, nil

	case schema.UsersetRewriteComputedUserset:
		// Check computed userset (another relation on the same object)
		if rewrite.ComputedUserset == nil {
			return false, fmt.Errorf("computed_userset is nil")
		}
		return e.EvaluateUserset(objectID, rewrite.ComputedUserset.Relation, subject)

	case schema.UsersetRewriteTupleToUserset:
		// Check tuple_to_userset (relation on another object)
		if rewrite.TupleToUserset == nil {
			return false, fmt.Errorf("tuple_to_userset is nil")
		}

		// Get the tupleset relation
		tupleRelation := rewrite.TupleToUserset.Tupleset.Relation

		// Find all objects that have the specified relation with this object
		var relatedObjects []string
		for _, r := range e.store.relationships {
			if r.Resource == objectID && r.Relation == tupleRelation {
				relatedObjects = append(relatedObjects, r.Subject)
			}
		}

		// Check if the subject has the computed relation with any of the related objects
		computedRelation := rewrite.TupleToUserset.ComputedUserset.Relation
		for _, relatedObj := range relatedObjects {
			allowed, err := e.EvaluateUserset(relatedObj, computedRelation, subject)
			if err != nil {
				return false, err
			}
			if allowed {
				return true, nil
			}
		}
		return false, nil

	case schema.UsersetRewriteUnion:
		// Check union (any of the child rules match)
		if rewrite.Children == nil || len(rewrite.Children) == 0 {
			return false, fmt.Errorf("union has no children")
		}

		for _, child := range rewrite.Children {
			allowed, err := e.evaluateUsersetRewrite(objectID, child, subject)
			if err != nil {
				return false, err
			}
			if allowed {
				return true, nil
			}
		}
		return false, nil

	case schema.UsersetRewriteIntersection:
		// Check intersection (all of the child rules match)
		if rewrite.Children == nil || len(rewrite.Children) == 0 {
			return false, fmt.Errorf("intersection has no children")
		}

		for _, child := range rewrite.Children {
			allowed, err := e.evaluateUsersetRewrite(objectID, child, subject)
			if err != nil {
				return false, err
			}
			if !allowed {
				return false, nil
			}
		}
		return true, nil

	case schema.UsersetRewriteExclusion:
		// Check exclusion (base - subtract)
		if rewrite.Children == nil || len(rewrite.Children) != 2 {
			return false, fmt.Errorf("exclusion must have exactly 2 children")
		}

		baseAllowed, err := e.evaluateUsersetRewrite(objectID, rewrite.Children[0], subject)
		if err != nil {
			return false, err
		}

		if !baseAllowed {
			return false, nil
		}

		subtractAllowed, err := e.evaluateUsersetRewrite(objectID, rewrite.Children[1], subject)
		if err != nil {
			return false, err
		}

		return baseAllowed && !subtractAllowed, nil

	default:
		return false, fmt.Errorf("unknown userset rewrite type: %s", rewrite.Type)
	}
}
