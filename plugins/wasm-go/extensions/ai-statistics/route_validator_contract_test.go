package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeTrustedRouteNameExactEmpty(t *testing.T) {
	require.Equal(t, "route_unknown", normalizeTrustedRouteName(""))
}
