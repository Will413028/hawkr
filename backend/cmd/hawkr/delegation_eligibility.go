package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

func parseDelegationEligibility(args []string) (delegation.AdminEligibilityChange, error) {
	var in delegation.AdminEligibilityChange
	if len(args) == 0 || (args[0] != "enable" && args[0] != "disable") {
		return in, errors.New("用法：delegation-eligibility enable|disable -admin-id <ID> -operator-admin-id <ID>")
	}
	in.Enabled = args[0] == "enable"
	flags := flag.NewFlagSet("delegation-eligibility", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Int64Var(&in.AdminID, "admin-id", 0, "管理員 ID")
	flags.Int64Var(&in.OperatorAdminID, "operator-admin-id", 0, "執行者管理員 ID")
	if err := flags.Parse(args[1:]); err != nil {
		return in, fmt.Errorf("delegation-eligibility 參數：%w", err)
	}
	if in.AdminID <= 0 || in.OperatorAdminID <= 0 || flags.NArg() != 0 {
		return in, errors.New("delegation-eligibility 需要正整數 admin-id、operator-admin-id，且不能有多餘參數")
	}
	return in, nil
}

func runDelegationEligibility(ctx context.Context, args []string) error {
	in, err := parseDelegationEligibility(args)
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
	log := newLogger()
	id := identity.New(identity.Deps{DB: d, Now: time.Now, Logger: log})
	del := delegation.New(delegation.Deps{DB: d, Identity: id, Seller: seller.NewReader(d, time.Now), Now: time.Now, Logger: log})
	return del.SetAdminEligibility(ctx, in)
}
