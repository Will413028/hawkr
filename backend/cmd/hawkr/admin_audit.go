package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const adminAuditUsage = "用法：admin-audit [-admin <ID>] [-action <procedure 名稱>] [-target-type <種類>] [-target-id <ID>] [-since <時間>] [-until <時間>] [-limit <筆數>]（時間是 RFC 3339 或 YYYY-MM-DD，Since 含、Until 不含）"

// parseAdminAudit 解析 admin-audit 的旗標（D62 的讀取端：只有營運子命令，沒有畫面）；全部選填，沒有條件就列最新的。
func parseAdminAudit(args []string) (adminaudit.Filter, error) {
	var f adminaudit.Filter
	flags := flag.NewFlagSet("admin-audit", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	admin := flags.Int64("admin", 0, "管理員 ID")
	action := flags.String("action", "", "動作（後台 RPC 的 procedure 名稱）")
	targetType := flags.String("target-type", "", "對象種類，例如 payment_account")
	targetID := flags.String("target-id", "", "對象 ID（要搭配 -target-type）")
	since := flags.String("since", "", "起（含）")
	until := flags.String("until", "", "迄（不含）")
	limit := flags.Int("limit", 0, "最多幾筆（預設 100，上限 1000）")
	if err := flags.Parse(args); err != nil {
		return f, fmt.Errorf("admin-audit 參數：%w（%s）", err, adminAuditUsage)
	}
	if flags.NArg() != 0 {
		return f, fmt.Errorf("admin-audit 不能有多餘參數（%s）", adminAuditUsage)
	}
	if *admin < 0 || *limit < 0 {
		return f, errors.New("admin-audit 的 -admin 與 -limit 不得為負")
	}
	if *targetID != "" && *targetType == "" {
		return f, errors.New("admin-audit 的 -target-id 要搭配 -target-type")
	}
	var err error
	if f.Since, err = parseAuditTime("-since", *since); err != nil {
		return f, err
	}
	if f.Until, err = parseAuditTime("-until", *until); err != nil {
		return f, err
	}
	if !f.Since.IsZero() && !f.Until.IsZero() && !f.Until.After(f.Since) {
		return f, errors.New("admin-audit 的 -until 要晚於 -since")
	}
	f.AdminID, f.Action, f.TargetType, f.TargetID, f.Limit = *admin, *action, *targetType, *targetID, *limit
	return f, nil
}

func parseAuditTime(flagName, v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, time.DateOnly} {
		if t, err := time.ParseInLocation(layout, v, time.UTC); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("admin-audit 的 %s %q 不是 RFC 3339 或 YYYY-MM-DD（%s）", flagName, v, adminAuditUsage)
}

// auditLister 是 adminaudit.Log 的讀取。
type auditLister interface {
	List(context.Context, adminaudit.Filter) ([]adminaudit.Record, error)
}

// printAdminAudit 列出紀錄，一筆一行（新的在前）：id、時間（UTC）、管理員、動作、對象、detail。
func printAdminAudit(ctx context.Context, l auditLister, f adminaudit.Filter, out io.Writer) error {
	recs, err := l.List(ctx, f)
	if err != nil {
		return err
	}
	for _, r := range recs {
		if _, err := fmt.Fprintf(out, "%d\t%s\tadmin=%d\t%s\t%s/%s\t%s\n", r.ID, r.OccurredAt.UTC().Format(time.RFC3339), r.AdminID, r.Action, r.TargetType, r.TargetID, r.Detail); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(out, "共 %d 筆\n", len(recs))
	return err
}

func runAdminAudit(ctx context.Context, args []string, out io.Writer) error {
	f, err := parseAdminAudit(args)
	if err != nil {
		return err
	}
	e := config.NewEnv()
	dbc := dbConfig(e)
	if err := e.Err(); err != nil {
		return err
	}
	d, err := db.Open(ctx, dbc)
	if err != nil {
		return err
	}
	defer d.Close()
	return printAdminAudit(ctx, adminaudit.New(d, time.Now), f, out)
}
