package cropidentity

import (
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/stretchr/testify/require"
)

func TestNewResolverForEvaluation_BuildsSameTypeAsProduction(t *testing.T) {
	r := NewResolverForEvaluation(nil, llmprovider.Procurement{})
	require.NotNil(t, r)
	require.IsType(t, &DisambiguationUseCase{}, r)
}
