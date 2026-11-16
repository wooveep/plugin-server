package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// #13: compare the actual fixed-code producers with the access-log boundary,
// rather than maintaining another partial copy of the allow-list in the test.
func TestResponsesIncidentControlledReasonCallersAreCovered(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	arguments := map[string]int{"managedFactGap": 1, "rejectManagedRequest": 2, "rejectManagedPreparation": 3, "recordManagedAccessFailure": 1}
	files := token.NewFileSet()
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(files, entry.Name(), nil, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			index, producer := arguments[name.Name]
			if !producer || len(call.Args) <= index {
				return true
			}
			literal, ok := call.Args[index].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			reason, err := strconv.Unquote(literal.Value)
			require.NoError(t, err)
			if safeManagedAccessReason(reason) != reason {
				t.Errorf("%s: fixed reason %q is lost at the access-log boundary", files.Position(literal.Pos()), reason)
			}
			return true
		})
	}
	require.Equal(t, "other_managed_failure", safeManagedAccessReason("tool_uncontrolled_vendor_detail"))
}
