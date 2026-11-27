package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/user"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

const adminRoleUsage = "用法：admin-role grant|revoke -admin <ID 或 email> -role <角色> -reason <理由>"

type adminRoleAction string

const (
	adminRoleGrant  adminRoleAction = "grant"
	adminRoleRevoke adminRoleAction = "revoke"
)

func parseAdminRole(args []string, osUser string) (adminRoleAction, identity.AdminRoleChangeInput, error) {
	var in identity.AdminRoleChangeInput
	if len(args) == 0 || (args[0] != string(adminRoleGrant) && args[0] != string(adminRoleRevoke)) {
		return "", in, errors.New(adminRoleUsage)
	}
	action := adminRoleAction(args[0])
	flags := flag.NewFlagSet("admin-role", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	admin := flags.String("admin", "", "管理員的 ID 或 email（要先以 Access 登入過一次）")
	role := flags.String("role", "", "角色，目前只有 "+string(adminperm.RoleOwner))
	reason := flags.String("reason", "", "理由（必填）")
	if err := flags.Parse(args[1:]); err != nil {
		return "", in, fmt.Errorf("admin-role 參數：%w（%s）", err, adminRoleUsage)
	}
	if flags.NArg() != 0 {
		return "", in, fmt.Errorf("admin-role 不能有多餘參數（%s）", adminRoleUsage)
	}
	switch id, err := strconv.ParseInt(*admin, 10, 64); {
	case err == nil && id > 0:
		in.AdminID = id
	case err == nil:
		return "", in, errors.New("admin-role 的 -admin 的 ID 必須是正整數")
	case strings.Contains(*admin, "@"):
		in.Email = strings.TrimSpace(*admin)
	default:
		return "", in, errors.New("admin-role 需要 -admin：管理員的 ID 或 email")
	}
	if *role == "" {
		return "", in, fmt.Errorf("admin-role 需要 -role（%v）", adminperm.Roles())
	}
	in.Role = adminperm.Role(*role)
	if !slices.Contains(adminperm.Roles(), in.Role) {
		return "", in, fmt.Errorf("admin-role 的 -role %q 不是角色（%v）", *role, adminperm.Roles())
	}
	if strings.TrimSpace(*reason) == "" {
		return "", in, errors.New("admin-role 需要 -reason：說明為什麼授予或撤銷")
	}
	in.Reason = *reason
	in.Source = identity.AdminRoleSourceCLI
	in.OSUser = osUser
	return action, in, nil
}

type adminRoleAPI interface {
	GrantAdminRole(context.Context, identity.AdminRoleChangeInput) (identity.AdminRoleChange, error)
	RevokeAdminRole(context.Context, identity.AdminRoleChangeInput) (identity.AdminRoleChange, error)
}

func applyAdminRole(ctx context.Context, api adminRoleAPI, action adminRoleAction, in identity.AdminRoleChangeInput, out io.Writer) error {
	if action == adminRoleGrant {
		res, err := api.GrantAdminRole(ctx, in)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "已授予管理員 %d 角色 %s\n", res.AdminID, res.Role)
		return err
	}
	res, err := api.RevokeAdminRole(ctx, in)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "已撤銷管理員 %d 的角色 %s；剩下的角色不再允許開始代行時，他進行中的代行已一併撤銷\n", res.AdminID, res.Role); err != nil {
		return err
	}
	if !res.HolderRemains {
		_, err = fmt.Fprintf(out, "警告：現在沒有任何管理員持有 %s；要再授予請執行 admin-role grant\n", res.Role)
	}
	return err
}

func runAdminRole(ctx context.Context, args []string, out io.Writer) error {
	osUser := ""
	if u, err := user.Current(); err == nil {
		osUser = u.Username
	}
	action, in, err := parseAdminRole(args, osUser)
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
	bus := events.NewBus()
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	if _, err := newDelegationOffline(d, bus, id, log); err != nil {
		return err
	}
	return applyAdminRole(ctx, id, action, in, out)
}
