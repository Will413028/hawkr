//go:build integration

package characterization

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/media/v1alpha1/mediav1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

type connectDriver struct {
	http    *http.Client
	base    string
	auth    func(ch.Caller, http.Header)
	clients map[ch.Caller]mediav1alpha1connect.MediaAppServiceClient
}

var _ MonolithDriver = (*connectDriver)(nil)

var anyMD5 = base64.StdEncoding.EncodeToString(make([]byte, 16))

func newConnectDriver(httpClient *http.Client, base string, auth func(ch.Caller, http.Header)) *connectDriver {
	d := &connectDriver{http: httpClient, base: base, auth: auth, clients: map[ch.Caller]mediav1alpha1connect.MediaAppServiceClient{}}
	for _, c := range []ch.Caller{ch.Anonymous, ch.Buyer, ch.Buyer2, ch.Admin} {
		interceptor := connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				auth(c, req.Header())
				return next(ctx, req)
			}
		}))
		d.clients[c] = mediav1alpha1connect.NewMediaAppServiceClient(httpClient, base, interceptor)
	}
	return d
}

func (d *connectDriver) Variant() Variant { return Monolith }

func (d *connectDriver) CreateUploadSession(ctx context.Context, c ch.Caller, in UploadInput) (UploadSession, error) {
	size, err := strconv.ParseInt(in.FileSize, 10, 64)
	if err != nil {
		return UploadSession{}, apperr.WithCause(ch.ErrInvalid, err)
	}
	if in.IsSecure {
		s, err := d.BeginSecureUpload(ctx, c, size)
		if err != nil {
			return UploadSession{}, err
		}
		return UploadSession{ID: s.Token, IsSecure: true}, nil
	}
	resp, err := d.clients[c].BeginUpload(ctx, connect.NewRequest(&pb.BeginUploadRequest{Size: size, ContentType: in.ContentType, Md5: anyMD5}))
	if err != nil {
		return UploadSession{}, ch.ConnectKind(err)
	}
	remaining := time.Until(resp.Msg.GetExpireTime().AsTime()).Round(time.Second)
	return UploadSession{
		ID: resp.Msg.GetStagedToken(), UploadURL: resp.Msg.GetUploadUrl(), ExpiresIn: strconv.FormatInt(int64(remaining/time.Second), 10),
	}, nil
}

func (d *connectDriver) CompleteUploadSession(ctx context.Context, c ch.Caller, sessionID string) (CompletedSession, error) {
	if _, err := d.clients[c].CompleteUpload(ctx, connect.NewRequest(&pb.CompleteUploadRequest{StagedToken: sessionID})); err != nil {
		return CompletedSession{}, ch.ConnectKind(err)
	}
	got, err := d.clients[c].GetUpload(ctx, connect.NewRequest(&pb.GetUploadRequest{StagedToken: sessionID}))
	if err != nil {
		return CompletedSession{}, ch.ConnectKind(err)
	}
	state := got.Msg.GetState().String()
	return CompletedSession{StagedToken: sessionID, State: state[len("UPLOAD_STATE_"):]}, nil
}

func (d *connectDriver) BeginSecureUpload(ctx context.Context, c ch.Caller, size int64) (SecureSession, error) {
	resp, err := d.clients[c].BeginSecureUpload(ctx, connect.NewRequest(&pb.BeginSecureUploadRequest{Size: size}))
	if err != nil {
		return SecureSession{}, ch.ConnectKind(err)
	}
	return SecureSession{Token: resp.Msg.GetStagedToken(), ChunkSize: int(resp.Msg.GetChunkSize()), ChunkCount: int(resp.Msg.GetChunkCount())}, nil
}

func (d *connectDriver) PutSecureChunk(ctx context.Context, c ch.Caller, token string, index int, data []byte) (int, error) {
	url := fmt.Sprintf("%s/api/files/secure_uploads/%s/chunks/%d", d.base, token, index)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	d.auth(c, req.Header)
	resp, err := d.http.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}
