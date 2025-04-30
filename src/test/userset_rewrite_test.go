package test

import (
	"fmt"
	"testing"

	"github.com/kanywst/zanzibar/src/policy"
	"github.com/kanywst/zanzibar/src/schema"
)

func TestUsersetRewriteRules(t *testing.T) {
	// Initialize schema with userset rewrite rules
	schemaStore := schema.LoadDefaultSchema()
	err := schemaStore.UpdateDefinitionWithUsersetRewrites()
	if err != nil {
		t.Fatalf("Failed to update schema with userset rewrite rules: %v", err)
	}

	// Initialize policy store with sample data
	policyStore := policy.NewStore(schemaStore)
	policyStore.InitializeWithSampleData()

	// Test cases
	testCases := []struct {
		name     string
		subject  string
		resource string
		action   string
		expected bool
	}{
		{
			name:     "Direct owner can view document",
			subject:  "user:alice",
			resource: "document:report",
			action:   "view",
			expected: true,
		},
		{
			name:     "Direct editor can view document",
			subject:  "user:bob",
			resource: "document:report",
			action:   "view",
			expected: true,
		},
		{
			name:     "Direct group member can view document",
			subject:  "user:charlie",
			resource: "document:report",
			action:   "view",
			expected: true,
		},
		{
			name:     "Nested group member can view document",
			subject:  "user:dave",
			resource: "document:report",
			action:   "view",
			expected: true,
		},
		{
			name:     "Parent folder viewer can view document (tuple_to_userset)",
			subject:  "user:eve",
			resource: "document:report",
			action:   "view",
			expected: true,
		},
		{
			name:     "Unknown user cannot view document",
			subject:  "user:frank",
			resource: "document:report",
			action:   "view",
			expected: false,
		},
		{
			name:     "Owner can edit document",
			subject:  "user:alice",
			resource: "document:report",
			action:   "edit",
			expected: true,
		},
		{
			name:     "Editor can edit document",
			subject:  "user:bob",
			resource: "document:report",
			action:   "edit",
			expected: true,
		},
		{
			name:     "Viewer cannot edit document",
			subject:  "user:eve",
			resource: "document:report",
			action:   "edit",
			expected: false,
		},
		{
			name:     "Only owner can delete document",
			subject:  "user:alice",
			resource: "document:report",
			action:   "delete",
			expected: true,
		},
		{
			name:     "Editor cannot delete document",
			subject:  "user:bob",
			resource: "document:report",
			action:   "delete",
			expected: false,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed, reason, err := policyStore.Check(tc.subject, tc.resource, tc.action)
			if err != nil {
				t.Fatalf("Check failed: %v", err)
			}

			if allowed != tc.expected {
				t.Errorf("Expected %v, got %v. Reason: %s", tc.expected, allowed, reason)
			} else {
				fmt.Printf("✓ %s: %s\n", tc.name, reason)
			}
		})
	}
}

func TestUsersetRewriteExplanation(t *testing.T) {
	fmt.Println("\nUserset Rewrite Rules Explanation:")
	fmt.Println("----------------------------------")
	fmt.Println("1. this: Direct relationships from stored relation tuples")
	fmt.Println("   Example: user:alice is directly an owner of document:report")
	fmt.Println("")
	fmt.Println("2. computed_userset: Computed from another relation on the same object")
	fmt.Println("   Example: All owners are also editors (owner → editor)")
	fmt.Println("   Example: All editors are also viewers (editor → viewer)")
	fmt.Println("")
	fmt.Println("3. tuple_to_userset: Computed from a relation on another object")
	fmt.Println("   Example: document:report has parent folder:projects")
	fmt.Println("   Example: user:eve is a viewer of folder:projects")
	fmt.Println("   Example: Therefore, user:eve is a viewer of document:report")
	fmt.Println("")
	fmt.Println("4. union: Any of the child rules match")
	fmt.Println("   Example: viewer = this | editor | parent#viewer")
	fmt.Println("")
	fmt.Println("5. intersection: All of the child rules match")
	fmt.Println("   Example: admin = owner & member")
	fmt.Println("")
	fmt.Println("6. exclusion: Base minus subtract")
	fmt.Println("   Example: collaborator = viewer - blocked")
}
