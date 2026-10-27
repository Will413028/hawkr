package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// AccountClient is the gRPC client for account service
type AccountClient struct {
	conn   *grpc.ClientConn
	client pb.AccountServiceClient
	// delegation shares the same connection: DelegationService is served by
	// account-service, so a second dial would only add a second failure mode.
	delegation pb.DelegationServiceClient
}

// NewAccountClient creates a new account gRPC client
func NewAccountClient() (*AccountClient, error) {
	// Default port for account-service
	host := getEnvOrDefault("ACCOUNT_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("ACCOUNT_SERVICE_GRPC_PORT", "9941")
	target := fmt.Sprintf("%s:%s", host, port)

	// Use a shorter timeout to avoid blocking too long during request
	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to account service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to account gRPC service at %s", target)

	return &AccountClient{
		conn:       conn,
		client:     pb.NewAccountServiceClient(conn),
		delegation: pb.NewDelegationServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection
func (c *AccountClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetAccountByPhone retrieves account info by phone number.
func (c *AccountClient) GetAccountByPhone(ctx context.Context, phone string) (*pb.AccountInfoResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.client.GetAccountByPhone(ctx, &pb.GetAccountByPhoneRequest{Phone: phone})
}

// GetCurrentDelegationSession resolves an operation credential into its session
// snapshot (56 §6.1.0 / §6.2.4a).
//
// countsAsActivity must be decided by WHICH middleware the route was mounted under,
// never by the handler and never by anything in the HTTP request: it is true for the
// two guards that sit on real operator actions (delegatedHandle 的 20 支資源路由、
// requireDelegatedSessionOnly 的兩支媒體端點) and false for the six session
// endpoints. GET current is polled every 30 seconds — counting a poll as activity
// means the idle window can never close while the workspace tab is open.
//
// deadlineMs <= 0 leaves the caller's deadline untouched (test holders carry no
// config); production always passes DELEGATION_GRPC_DEADLINE_CURRENT_MS.
func (c *AccountClient) GetCurrentDelegationSession(ctx context.Context, credential string, countsAsActivity bool, deadlineMs int) (*pb.GetCurrentSessionResponse, error) {
	if deadlineMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(deadlineMs)*time.Millisecond)
		defer cancel()
	}
	return c.delegation.GetCurrentSession(ctx, &pb.GetCurrentSessionRequest{
		OperationCredential: credential,
		CountsAsActivity:    countsAsActivity,
	})
}

// withDelegationDeadline applies a per-hop deadline. deadlineMs <= 0 leaves the
// caller's deadline untouched — test holders carry no config, and a zero there must
// not turn into an already-expired context.
func withDelegationDeadline(ctx context.Context, deadlineMs int) (context.Context, context.CancelFunc) {
	if deadlineMs <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, time.Duration(deadlineMs)*time.Millisecond)
}

// DelegationPreflight asks whether this administrator may start delegating for this
// store, and with which capabilities (56 S1).
//
// Read-only and safe to repeat: the entry point re-fetches it every 30 seconds, on
// window focus, and once more immediately before every create.
func (c *AccountClient) DelegationPreflight(ctx context.Context, req *pb.PreflightRequest, deadlineMs int) (*pb.PreflightResponse, error) {
	ctx, cancel := withDelegationDeadline(ctx, deadlineMs)
	defer cancel()

	return c.delegation.Preflight(ctx, req)
}

// CreateDelegationSession creates a session and a one-shot handoff code (56 S2).
//
// ⚠️ 回應內的 handoff_code 是明文憑證：不得進 log、
// Sentry 或 metric label，唯一合法去處是 S2 的回應 body（§8.1.6）。
func (c *AccountClient) CreateDelegationSession(ctx context.Context, req *pb.CreateSessionRequest, deadlineMs int) (*pb.CreateSessionResponse, error) {
	ctx, cancel := withDelegationDeadline(ctx, deadlineMs)
	defer cancel()

	return c.delegation.CreateSession(ctx, req)
}

// EndDelegationSession ends the session the credential belongs to (56 S4). Ending is
// "expire now", so it is idempotent and a credential that resolves to nothing is
// still a success.
func (c *AccountClient) EndDelegationSession(ctx context.Context, credential, requestID string, deadlineMs int) (*pb.EndSessionResponse, error) {
	ctx, cancel := withDelegationDeadline(ctx, deadlineMs)
	defer cancel()

	return c.delegation.EndSession(ctx, &pb.EndSessionRequest{
		OperationCredential: credential,
		RequestId:           requestID,
	})
}

// RevokeDelegationSession is the ADM governance action (56 S5): it may target another
// administrator's session, and an already-expired session may still be revoked so the
// governance trace exists.
func (c *AccountClient) RevokeDelegationSession(ctx context.Context, req *pb.RevokeSessionRequest, deadlineMs int) (*pb.RevokeSessionResponse, error) {
	ctx, cancel := withDelegationDeadline(ctx, deadlineMs)
	defer cancel()

	return c.delegation.RevokeSession(ctx, req)
}

// RedeemDelegationHandoff trades the one-shot handoff code for the credential (56 S6).
//
// All four protections (single use via one-statement CAS, 60-second window, only the
// code's own session, and terminal/expired sessions refused) live inside the RPC —
// none of them may be pushed onto this caller, because S6 has no other identity to
// check against.
//
// ⚠️ The response carries plaintext credential: never log it.
func (c *AccountClient) RedeemDelegationHandoff(ctx context.Context, handoffCode, requestID string, deadlineMs int) (*pb.RedeemHandoffResponse, error) {
	ctx, cancel := withDelegationDeadline(ctx, deadlineMs)
	defer cancel()

	return c.delegation.RedeemHandoff(ctx, &pb.RedeemHandoffRequest{
		HandoffCode: handoffCode,
		RequestId:   requestID,
	})
}
