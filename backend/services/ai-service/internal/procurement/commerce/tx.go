package commerce

import "context"

// TxRunner 是把數個 repository 呼叫綁成單一 transaction 的能力，形狀與
// go-pkg/database.TxManager 完全相同（*pgxTxManager 直接滿足它，不需要 adapter）。
// 在這裡另外宣告一次而不是直接 import pkg/database，理由同本套件其餘幾個 port：
// commerce 層只宣告自己需要的能力，把「誰來實作」留給組合根（procurement/module.go
// 已經 provide 了 database.TxManager），這一層不必因為一個一行介面就相依基礎設施套件。
// 交易靠 ctx 傳遞：WithTx 把 pgx.Tx 塞進 ctx，repository 端的
// database.GetDBTX(ctx, pool) 就會自動改用該交易，
// 所以三支 repository 方法的簽名完全不用動。
type TxRunner interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
