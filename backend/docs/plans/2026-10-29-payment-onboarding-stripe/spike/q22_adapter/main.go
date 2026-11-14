//go:build spike

// q22_adapter 用正式的 adapter（internal/platform/stripe）對 Stripe sandbox 走一次：
// create（兩步建立＋開通連結）、shell（只做第一步的空帳戶）、find（以 metadata.store_id 列出比對）、status（讀帳戶）、close（關閉，連關兩次確認冪等）。
// 用法：go run -tags spike ./docs/plans/2026-10-29-payment-onboarding-stripe/spike/q22_adapter <create [none]|shell|find|link|status|close> [account id]
// create 後面加 none 是平台收件（dashboard=none，Q23），不加是 Stripe 收件（express，Q22）。
// 金鑰從環境變數 STRIPE_SECRET_KEY 讀，命名空間從 STRIPE_ACCOUNT_NAMESPACE 讀（沒設時用 spike；create、shell、find 帶它）。
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/Will413028/hawkr/backend/internal/platform/stripe"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func namespace() string {
	if ns := os.Getenv("STRIPE_ACCOUNT_NAMESPACE"); ns != "" {
		return ns
	}
	return "spike"
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: q22_adapter <create|status|close> [account id]")
	}
	c, err := stripe.New(stripe.Config{SecretKey: os.Getenv("STRIPE_SECRET_KEY"), WebhookSecret: "whsec_unused", AccountNamespace: namespace()})
	if err != nil {
		return err
	}
	ctx := context.Background()
	switch os.Args[1] {
	case "create":
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			return err
		}
		key := "q22-" + hex.EncodeToString(raw)
		dashboard := stripe.DashboardExpress
		if len(os.Args) > 2 && os.Args[2] == "none" {
			dashboard = stripe.DashboardNone
		}
		id, err := c.CreateAccount(ctx, key, 990022, dashboard, "seller-q22@example.com")
		if err != nil {
			return fmt.Errorf("CreateAccount: %w", err)
		}
		fmt.Println("account:", id)
		link, err := c.CreateAccountLink(ctx, id, "https://example.com/return", "https://example.com/refresh")
		if err != nil {
			return fmt.Errorf("CreateAccountLink: %w", err)
		}
		fmt.Println("link:", link.URL)
		fmt.Println("expires:", link.ExpiresAt)
	case "link":
		link, err := c.CreateAccountLink(ctx, os.Args[2], "https://example.com/return", "https://example.com/refresh")
		if err != nil {
			return fmt.Errorf("CreateAccountLink: %w", err)
		}
		fmt.Println("link:", link.URL)
	case "shell":
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			return err
		}
		id, err := c.CreateAccountShell(ctx, "q22-"+hex.EncodeToString(raw), 990027)
		if err != nil {
			return fmt.Errorf("CreateAccountShell: %w", err)
		}
		fmt.Println("account:", id)
	case "find":
		ids, err := c.ListAccountIDsByStore(ctx, 990027)
		if err != nil {
			return fmt.Errorf("ListAccountIDsByStore: %w", err)
		}
		fmt.Println("found:", ids)
	case "status":
		a, err := c.RetrieveAccount(ctx, os.Args[2])
		if err != nil {
			return fmt.Errorf("RetrieveAccount: %w", err)
		}
		fmt.Printf("transfers: %+v\npayouts: %+v\nrequirements: %s\n", a.Transfers, a.Payouts, a.Requirements)
	case "close":
		for i := 1; i <= 2; i++ {
			if err := c.CloseAccount(ctx, os.Args[2]); err != nil {
				return fmt.Errorf("CloseAccount #%d: %w", i, err)
			}
			fmt.Printf("close #%d: ok\n", i)
		}
		_, err := c.RetrieveAccount(ctx, os.Args[2])
		fmt.Println("retrieve after close:", err)
	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
	return nil
}
