package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type cleanupStub struct {
	accounts []int64
	err      error
}

func (s *cleanupStub) DeleteByAccountID(_ context.Context, id int64) error {
	s.accounts = append(s.accounts, id)
	return s.err
}

func TestSellerDeletedUsesAccountIDAndReplays(t *testing.T) {
	repo := &cleanupStub{}
	handler := newSellerDeletedHandler(repo, zap.NewNop())
	body := []byte(`{"event_type":"SELLER_DELETED","accountId":777,"store_id":100}`)
	for range 2 {
		require.NoError(t, handler(context.Background(), mq.RoutingKeySellerDeleted, body))
	}
	require.Equal(t, []int64{777, 777}, repo.accounts)
	// No store identity is needed for an account-scoped deletion.
	require.NoError(t, handler(context.Background(), mq.RoutingKeySellerDeleted, []byte(`{"accountId":888}`)))
	require.Equal(t, int64(888), repo.accounts[2])
}

func TestSellerDeletedRejectsInvalidPayload(t *testing.T) {
	for _, body := range []string{`null`, `{}`, ``, `{`, `[]`, `{"store_id":777}`, `{"accountId":0}`, `{"accountId":-1}`, `{"accountId":"777"}`} {
		t.Run(body, func(t *testing.T) {
			repo := &cleanupStub{}
			err := newSellerDeletedHandler(repo, zap.NewNop())(context.Background(), mq.RoutingKeySellerDeleted, []byte(body))
			require.Error(t, err)
			require.Empty(t, repo.accounts)
		})
	}
}

func TestSellerDeletedPropagatesDatabaseFailure(t *testing.T) {
	dbErr := errors.New("database unavailable")
	repo := &cleanupStub{err: dbErr}
	err := newSellerDeletedHandler(repo, zap.NewNop())(context.Background(), mq.RoutingKeySellerDeleted, []byte(`{"accountId":777,"store_id":100}`))
	require.ErrorIs(t, err, dbErr)
	require.Equal(t, []int64{777}, repo.accounts)
}
