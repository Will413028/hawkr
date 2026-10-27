package router

import (
	"testing"
)

// PATCH /api/v1/app/sellers/:id 是 #2813 移除的死路由，不得復活。
//
// 它為什麼該死：
//   - 下游沒有實作 —— seller-service 沒有 /app/sellers/{id} handler，pod 內直打
//     該路徑得到的是 404。
//   - 前端沒有呼叫端 —— web / admin / buyer / seller 四個 repo 全數零命中。
//   - 而它同時是個地雷：走裸 JWTAuth 的 CreateProxy 代理，繞過了 46 §3.8 的
//     capability 宣告，等於在停權判定上開一個洞。
func TestDeadSellerPatchProxyRouteStaysRemoved(t *testing.T) {
	r := buildFullRouter(t)

	for _, ri := range r.Routes() {
		if ri.Method == "PATCH" && ri.Path == appPrefix+"/sellers/:id" {
			t.Fatalf("死路由復活了：%s %s —— 下游未實作且無前端消費者，"+
				"且裸 JWTAuth 代理會繞過 capability 宣告（#2813）", ri.Method, ri.Path)
		}
	}
}
