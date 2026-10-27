package publicationoutbox

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeConn 是 lazyRawPublisher 連上之後拿到的東西。
type fakeConn struct {
	calls  []rawCallRecord
	closed bool
}

type rawCallRecord struct {
	exchange   string
	routingKey string
	body       string
}

func (c *fakeConn) PublishRaw(_ context.Context, exchange, routingKey string, body []byte) error {
	c.calls = append(c.calls, rawCallRecord{exchange: exchange, routingKey: routingKey, body: string(body)})
	return nil
}

func (c *fakeConn) Close() error { c.closed = true; return nil }

// ⚠️ 本檔沒有 //go:build integration：這幾支不需要 DB／broker，跑在一般 unit 測試
// 那一輪（`go test ./...`），CI 不必開 docker 就守得住自癒這條性質。

// TestLazyRawPublisher_RetriesConnectAfterEarlierFailure 是這次修正的核心證據。
//
// 場景：ai-service 滾動重啟時 broker 剛好不可用，兩分鐘後恢復。
//   - 現在的實作：第一次發送連線失敗並回可重試的錯誤，**沒有把失敗記在任何地方**，
//     所以下一輪重試會再連一次、成功之後照常送出。
//   - 先前那版（DI 時就連、連不上回 nil、之後整個 tick 跳過）：這一列永遠不會再被
//     嘗試，要等有人重啟 pod。這支測試在那版下必紅——它根本走不到第二次連線。
func TestLazyRawPublisher_RetriesConnectAfterEarlierFailure(t *testing.T) {
	conn := &fakeConn{}
	attempts := 0
	p := &lazyRawPublisher{
		logger: zap.NewNop(),
		connect: func() (rawConn, error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("dial tcp: connection refused")
			}
			return conn, nil
		},
	}

	err := p.PublishRaw(context.Background(), "procurement.events", "k", []byte(`{}`))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPublisherUnavailable,
		"連不上必須是可重試的錯誤，worker 才會照 backoff 重排這一列")
	require.Empty(t, conn.calls)

	require.NoError(t, p.PublishRaw(context.Background(), "procurement.events", "k", []byte(`{"a":1}`)))
	require.Equal(t, 2, attempts, "第二次呼叫必須重新嘗試連線，而不是記住上次失敗")
	require.Len(t, conn.calls, 1)
	require.Equal(t, rawCallRecord{exchange: "procurement.events", routingKey: "k", body: `{"a":1}`}, conn.calls[0])
}

// 連上之後不再重連——每一列都重開一條連線會把 broker 打爆。
func TestLazyRawPublisher_ReusesConnectionOnceEstablished(t *testing.T) {
	conn := &fakeConn{}
	attempts := 0
	p := &lazyRawPublisher{
		logger:  zap.NewNop(),
		connect: func() (rawConn, error) { attempts++; return conn, nil },
	}

	for i := 0; i < 3; i++ {
		require.NoError(t, p.PublishRaw(context.Background(), "procurement.events", "k", []byte(`{}`)))
	}
	require.Equal(t, 1, attempts)
	require.Len(t, conn.calls, 3)
}

// close 之後回到未連線狀態：下一次發送重新連一條，且重複 close 不 panic。
func TestLazyRawPublisher_CloseReleasesConnection(t *testing.T) {
	conn := &fakeConn{}
	attempts := 0
	p := &lazyRawPublisher{
		logger:  zap.NewNop(),
		connect: func() (rawConn, error) { attempts++; return conn, nil },
	}

	require.NoError(t, p.PublishRaw(context.Background(), "procurement.events", "k", []byte(`{}`)))
	require.NoError(t, p.close())
	require.True(t, conn.closed)
	require.NoError(t, p.close(), "重複 close 是安全的（fx OnStop 可能與其他關機路徑重疊）")

	require.NoError(t, p.PublishRaw(context.Background(), "procurement.events", "k", []byte(`{}`)))
	require.Equal(t, 2, attempts)
}

// 沒連上時，錯誤必須裹著 ErrPublisherUnavailable 一路傳回 PublishFunc 的呼叫端，
// worker 才分得出「這是可重試的管道問題」。
func TestNewPublishFunc_PropagatesPublisherUnavailable(t *testing.T) {
	p := &lazyRawPublisher{
		logger:  zap.NewNop(),
		connect: func() (rawConn, error) { return nil, errors.New("connection refused") },
	}
	err := NewPublishFunc(p)(context.Background(), "RFQ_PUBLICATION_INVITE_CREATED", []byte(`{}`))
	require.ErrorIs(t, err, ErrPublisherUnavailable)
}
