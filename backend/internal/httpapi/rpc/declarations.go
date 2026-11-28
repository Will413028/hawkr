package rpc

import (
	"fmt"
	"maps"
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

// DeferredServices 是契約與宣告表已有、但 handler 在後續步驟才掛上的 service（值是掛上的步驟）：舊服務處理商品流量到 B4-6，
var DeferredServices = map[string]string{
	"hawkr.admin.catalog.v1alpha1.CatalogAdminService": "B4-6",
	"hawkr.catalog.v1alpha1.CatalogPublicService":      "B4-6",
	"hawkr.catalog.v1alpha1.SellerCatalogAppService":   "B4-6",
	"hawkr.catalog.v1alpha1.CatalogDelegatedService":   "B4-6",
}

// WithoutUnmountedDeferred 回傳 table 去掉 DeferredServices 裡還沒掛的 service 的宣告，給 CheckDeclarations 用：
// 延後的 service 沒掛時不要求宣告對得上；掛上後和其他 service 一樣「宣告＝掛載」，其他 service 沒掛仍然不能啟動。
func WithoutUnmountedDeferred[D any](table map[string]D, mounted []protoreflect.ServiceDescriptor) map[string]D {
	out := maps.Clone(table)
	for name := range DeferredServices {
		if slices.ContainsFunc(mounted, func(sd protoreflect.ServiceDescriptor) bool { return string(sd.FullName()) == name }) {
			continue
		}
		for procedure := range out {
			if strings.HasPrefix(procedure, "/"+name+"/") {
				delete(out, procedure)
			}
		}
	}
	return out
}
