package characterize

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"connectrpc.com/connect"

	mediapb "github.com/Will413028/hawkr/backend/internal/legacy/gen/media"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/media/mediaconnect"
)

// FakeObjectKey 是假的 media-service 轉正後給的 object key：同一個 staged token 永遠得到同一個 key，直接打
// 單體時模組的 media 替身也用它，兩種執行的 key 相同。
func FakeObjectKey(refType, token string) string {
	return "public/" + refType + "/" + token + ".jpg"
}

// ServeFakeMedia 在 addr 聽一個假的 media-service（gRPC、h2c），給經 gateway 執行時的舊服務與單體呼叫：轉正
// 一律成功，不連到任何外部的儲存空間。B1 把 media 搬進單體之前，所有要轉正圖片的模組都用它。
func ServeFakeMedia(addr string) (stop func(), err error) {
	if addr == "" {
		return nil, errors.New("缺少 " + EnvFakeMediaAddr + "：用 scripts/characterize.sh 執行")
	}
	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("假的 media-service 無法聽 %s：%w", addr, err)
	}
	_, h := mediaconnect.NewMediaServiceHandler(fakeMediaService{})
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
	srv.Protocols = new(http.Protocols)
	srv.Protocols.SetUnencryptedHTTP2(true)
	go func() { _ = srv.Serve(ln) }()
	return func() { _ = srv.Close() }, nil
}

type fakeMediaService struct {
	mediaconnect.UnimplementedMediaServiceHandler
}

func (fakeMediaService) FinalizeImages(_ context.Context, req *connect.Request[mediapb.FinalizeImagesRequest]) (*connect.Response[mediapb.FinalizeImagesResponse], error) {
	out := &mediapb.FinalizeImagesResponse{}
	for _, tok := range req.Msg.GetStagedTokens() {
		out.Assets = append(out.Assets, &mediapb.FinalizedAsset{
			StagedToken: tok, AssetId: fakeAssetID(tok), ObjectKey: FakeObjectKey(req.Msg.GetRefType(), tok),
		})
	}
	return connect.NewResponse(out), nil
}

func (fakeMediaService) LinkImages(context.Context, *connect.Request[mediapb.LinkImagesRequest]) (*connect.Response[mediapb.LinkImagesResponse], error) {
	return connect.NewResponse(&mediapb.LinkImagesResponse{LinkedCount: "1"}), nil
}

func (fakeMediaService) UnlinkImages(context.Context, *connect.Request[mediapb.UnlinkImagesRequest]) (*connect.Response[mediapb.UnlinkImagesResponse], error) {
	return connect.NewResponse(&mediapb.UnlinkImagesResponse{UnlinkedCount: "1"}), nil
}

// fakeAssetID 是轉正後的 asset ID：全數字的 token 原樣當 asset ID（舊的 social-service 把 asset ID 當整數解析），
// 其他加上 asset- 前綴。
func fakeAssetID(token string) string {
	for _, r := range token {
		if r < '0' || r > '9' {
			return "asset-" + token
		}
	}
	if token == "" {
		return "asset-"
	}
	return token
}
