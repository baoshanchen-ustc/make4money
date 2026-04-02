package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeOpenCodeText_PreservesCanonicalSentence(t *testing.T) {
	in := "You are OpenCode, the best coding agent on the planet."
	got := sanitizeSystemText(in)
	require.Equal(t, in, got)
}
