// Package categoryservice 呼叫遷移期的 product-service 的分類服務（CategoryGrpcService）。
package categoryservice

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"

	categorypb "github.com/Will413028/hawkr/backend/internal/legacy/gen/category"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/category/categoryconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/grpcclient"
)

const (
	timeout = 10 * time.Second
	// maxDepth 擋住異常的深層或循環資料；商品分類只有三層。
	maxDepth = 8
)

// Client 讀取商品分類。
type Client struct {
	rpc categoryconnect.CategoryServiceClient
}

// Node 是分類樹的一個節點。
type Node struct {
	ID         int64
	Name       string
	ParentID   int64
	ParentName string
	Status     string
	Children   []Node
}

// New 建立以 h2c 呼叫並簽章的 client。
func New(addr, secret string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("categoryservice：缺少位址")
	}
	return &Client{rpc: categoryconnect.NewCategoryServiceClient(grpcclient.HTTPClient(), "http://"+addr, connect.WithGRPC(), connect.WithInterceptors(grpcclient.Signer(secret, nil)))}, nil
}

// CategoryTree 回傳只含啟用分類的分類樹。根節點的 ParentID 為 0。
func (c *Client) CategoryTree(ctx context.Context) ([]Node, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := c.rpc.GetCategoryTree(ctx, connect.NewRequest(&categorypb.GetCategoryTreeRequest{Status: "active"}))
	if err != nil {
		return nil, fmt.Errorf("categoryservice：GetCategoryTree：%w", err)
	}
	if resp.Msg.GetCode() != 0 {
		return nil, fmt.Errorf("categoryservice：GetCategoryTree code=%d", resp.Msg.GetCode())
	}
	return convert(resp.Msg.GetCategories(), 1)
}

func convert(nodes []*categorypb.CategoryNode, depth int) ([]Node, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("categoryservice：分類樹超過 %d 層", maxDepth)
	}
	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		id, err := strconv.ParseInt(n.GetId(), 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("categoryservice：分類 ID 不合法: %q", n.GetId())
		}
		var parent int64
		if raw := n.GetParentId(); raw != "" {
			if parent, err = strconv.ParseInt(raw, 10, 64); err != nil || parent < 0 {
				return nil, fmt.Errorf("categoryservice：父分類 ID 不合法: %q", raw)
			}
		}
		children, err := convert(n.GetChildren(), depth+1)
		if err != nil {
			return nil, err
		}
		out = append(out, Node{ID: id, Name: n.GetName(), ParentID: parent, ParentName: n.GetParentName(), Status: n.GetStatus(), Children: children})
	}
	return out, nil
}
