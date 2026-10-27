package rpc

import (
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// CheckDeclarations 雙向對照宣告表與 service 的方法（ADR 2026-10-04-admin-roles：「宣告＝掛載」）：每個方法都有宣告、
// 沒有指向不存在的方法的宣告，而且每個存在的方法的宣告都合法。table 的 key 是 procedure（/<service 全名>/<方法>）；
// invalid 回傳這筆宣告不合法的原因（接在 procedure 後面），合法時回空字串。呼叫方在建構 interceptor 時呼叫它，
// 有任何問題就不能啟動；scope 是錯誤訊息的前綴（例如 httpapi/seller）。
func CheckDeclarations[D any](scope string, table map[string]D, services []protoreflect.ServiceDescriptor, invalid func(D) string) error {
	var problems []string
	procedures := map[string]bool{}
	for _, svc := range services {
		methods := svc.Methods()
		for i := range methods.Len() {
			procedure := "/" + string(svc.FullName()) + "/" + string(methods.Get(i).Name())
			procedures[procedure] = true
			if _, ok := table[procedure]; !ok {
				problems = append(problems, procedure+" 沒有授權宣告")
			}
		}
	}
	for procedure, d := range table {
		if !procedures[procedure] {
			problems = append(problems, procedure+" 有宣告，但 service 沒有這個方法")
		} else if why := invalid(d); why != "" {
			problems = append(problems, procedure+" "+why)
		}
	}
	if len(problems) == 0 {
		return nil
	}
	slices.Sort(problems)
	return fmt.Errorf("%s：%s", scope, strings.Join(problems, "；"))
}
