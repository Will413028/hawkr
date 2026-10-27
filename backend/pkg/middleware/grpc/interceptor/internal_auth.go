package interceptor

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/go-pkg/config"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	HeaderInternalService = "x-internal-service"
	HeaderInternalTs      = "x-internal-ts"
	HeaderInternalNonce   = "x-internal-nonce"
	HeaderInternalSig     = "x-internal-sig"

	headerAccountID   = "x-account-id"
	headerAccountType = "x-account-type"
)

const (
	InternalAuthModeOff        = "off"
	InternalAuthModePermissive = "permissive"
	InternalAuthModeEnforcing  = "enforcing"
)

const (
	envInternalAuthMode    = "INTERNAL_AUTH_MODE"
	envInternalSecret      = "INTERNAL_SERVICE_HMAC_SECRET"
	envInternalSecretPrev  = "INTERNAL_SERVICE_HMAC_SECRET_PREV"
	envInternalTsWindow    = "INTERNAL_AUTH_TS_WINDOW_SECONDS"
	defaultTsWindowSeconds = 300
)

type internalAuthConfig struct {
	mode            string
	secret          string
	secretPrev      string
	tsWindowSeconds int64
}

func ResolveInternalAuthMode(raw string, present bool) (string, error) {
	if !present {
		return "", fmt.Errorf("%s 未設定；必須明確指定 %q／%q／%q 其中之一（沒有安全的預設值可回退）",
			envInternalAuthMode, InternalAuthModeOff, InternalAuthModePermissive, InternalAuthModeEnforcing)
	}
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case InternalAuthModeOff, InternalAuthModePermissive, InternalAuthModeEnforcing:
		return mode, nil
	default:
		return "", fmt.Errorf("%s=%q 無法辨識；必須是 %q／%q／%q 其中之一",
			envInternalAuthMode, raw, InternalAuthModeOff, InternalAuthModePermissive, InternalAuthModeEnforcing)
	}
}

func loadInternalAuthConfigE() (internalAuthConfig, error) {
	mode, err := ResolveInternalAuthMode(os.LookupEnv(envInternalAuthMode))
	if err != nil {
		return internalAuthConfig{}, err
	}
	return internalAuthConfig{
		mode:            mode,
		secret:          config.GetEnvOrDefault(envInternalSecret, ""),
		secretPrev:      config.GetEnvOrDefault(envInternalSecretPrev, ""),
		tsWindowSeconds: int64(config.GetEnvAsIntOrDefault(envInternalTsWindow, defaultTsWindowSeconds)),
	}, nil
}

func loadInternalAuthConfig() internalAuthConfig {
	cfg, err := loadInternalAuthConfigE()
	if err != nil {
		panic(fmt.Sprintf("internal auth 設定不合法：%v", err))
	}
	return cfg
}

func InternalAuthCanonical(serviceName, ts, nonce, accountID string) string {
	return serviceName + "\n" + ts + "\n" + nonce + "\n" + accountID
}

func InternalAuthSign(secret, canonical string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(canonical))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func newNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {

		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

func firstMD(md metadata.MD, key string) string {
	if md == nil {
		return ""
	}
	if vals := md.Get(key); len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func InternalAuthClientInterceptor(serviceName string, forwardAccountHeaders bool) grpc.UnaryClientInterceptor {
	cfg := loadInternalAuthConfig()
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		outMD, _ := metadata.FromOutgoingContext(ctx)
		outMD = outMD.Copy()

		if forwardAccountHeaders {
			if inMD, ok := metadata.FromIncomingContext(ctx); ok {
				if len(outMD.Get(headerAccountID)) == 0 {
					if v := firstMD(inMD, headerAccountID); v != "" {
						outMD.Set(headerAccountID, v)
					}
				}
				if len(outMD.Get(headerAccountType)) == 0 {
					if v := firstMD(inMD, headerAccountType); v != "" {
						outMD.Set(headerAccountType, v)
					}
				}
			}
		}

		accountID := firstMD(outMD, headerAccountID)

		outMD.Set(HeaderInternalService, serviceName)

		if cfg.secret != "" {
			ts := strconv.FormatInt(time.Now().Unix(), 10)
			nonce := newNonce()
			sig := InternalAuthSign(cfg.secret, InternalAuthCanonical(serviceName, ts, nonce, accountID))
			outMD.Set(HeaderInternalTs, ts)
			outMD.Set(HeaderInternalNonce, nonce)
			outMD.Set(HeaderInternalSig, sig)
		}

		ctx = metadata.NewOutgoingContext(ctx, outMD)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func InternalAuthServerInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	cfg := loadInternalAuthConfig()
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {

		if cfg.mode == InternalAuthModeOff {
			return handler(ctx, req)
		}

		md, _ := metadata.FromIncomingContext(ctx)
		caller := firstMD(md, HeaderInternalService)

		if caller == "" {
			return handler(ctx, req)
		}

		sig := firstMD(md, HeaderInternalSig)
		if sig == "" {
			if cfg.mode == InternalAuthModeEnforcing {
				return nil, status.Error(codes.Unauthenticated, "internal call missing signature")
			}
			if logger != nil {
				logger.Warn("internal.auth.unsigned",
					zap.String("caller", caller),
					zap.String("method", info.FullMethod))
			}
			return handler(ctx, req)
		}

		if verifyInternalAuth(cfg, md, caller) {
			return handler(ctx, req)
		}

		if cfg.mode == InternalAuthModeEnforcing {
			return nil, status.Error(codes.Unauthenticated, "internal call signature invalid")
		}
		if logger != nil {
			logger.Warn("internal.auth.invalid",
				zap.String("caller", caller),
				zap.String("method", info.FullMethod))
		}
		return handler(ctx, req)
	}
}

func verifyInternalAuth(cfg internalAuthConfig, md metadata.MD, caller string) bool {
	if cfg.secret == "" {

		return false
	}
	tsStr := firstMD(md, HeaderInternalTs)
	nonce := firstMD(md, HeaderInternalNonce)
	sig := firstMD(md, HeaderInternalSig)
	if tsStr == "" || sig == "" {
		return false
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return false
	}
	diff := time.Now().Unix() - ts
	if diff < 0 {
		diff = -diff
	}
	if diff > cfg.tsWindowSeconds {
		return false
	}
	accountID := firstMD(md, headerAccountID)
	canonical := InternalAuthCanonical(caller, tsStr, nonce, accountID)
	if hmacEqual(cfg.secret, canonical, sig) {
		return true
	}
	if cfg.secretPrev != "" && hmacEqual(cfg.secretPrev, canonical, sig) {
		return true
	}
	return false
}

func hmacEqual(secret, canonical, sigB64 string) bool {
	expected := InternalAuthSign(secret, canonical)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(sigB64)) == 1
}
