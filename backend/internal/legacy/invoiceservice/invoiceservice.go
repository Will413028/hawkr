// Package invoiceservice 呼叫遷移期的 invoice-service。隨 A5（invoice 搬進單體）刪除。
package invoiceservice

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	invoicepb "github.com/Will413028/hawkr/backend/internal/legacy/gen/invoice"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/invoice/invoiceconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/grpcclient"
)

const timeout = 10 * time.Second

const dateLayout = "2006-01-02"

// Client 推送行事曆鏡像給 invoice-service。
type Client struct {
	rpc invoiceconnect.InvoiceServiceClient
}

// New 建立以 h2c 呼叫並簽章的 client。
func New(addr, secret string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("invoiceservice：缺少位址")
	}
	return &Client{rpc: invoiceconnect.NewInvoiceServiceClient(grpcclient.HTTPClient(), "http://"+addr, connect.WithGRPC(), connect.WithInterceptors(grpcclient.Signer(secret, nil)))}, nil
}

// ReplaceBusinessDayMirror 以 [from, to]（台北日曆日，含兩端）與其中的放假日整段覆蓋 invoice 的鏡像。日期只看年月日。
func (c *Client) ReplaceBusinessDayMirror(ctx context.Context, from, to time.Time, holidays []time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req := &invoicepb.ReplaceBusinessDayMirrorRequest{
		CoveredFrom:  from.Format(dateLayout),
		CoveredTo:    to.Format(dateLayout),
		HolidayDates: make([]string, len(holidays)),
	}
	for i, h := range holidays {
		req.HolidayDates[i] = h.Format(dateLayout)
	}
	resp, err := c.rpc.ReplaceBusinessDayMirror(ctx, connect.NewRequest(req))
	if err != nil {
		return fmt.Errorf("invoiceservice：ReplaceBusinessDayMirror：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return fmt.Errorf("invoiceservice：ReplaceBusinessDayMirror code=%d", resp.Msg.GetCode())
	}
	return nil
}
