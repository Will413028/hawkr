package config

import (
	"strings"
	"testing"
)

// §4.5.8 環境 fail-closed 的鑑別力驗收。
//
// 這一支存在的理由：AMEGO 的**測試與正式共用同一個網域，回應也沒有任何環境標記**。
// 設定填錯不會連不上、不會回錯誤碼、log 裡看不出來——只會靜默開出一張性質完全相反
// 的真發票，而電子發票開出去就不可回收。⇒ 唯一擋得住它的是啟動時的斷言，而「斷言
// 存在」本身也需要被證明，否則它就只是一段沒人執行過的 code。
//
// 五條驗收對應 §4.5.8 的表：#1 #2 #3 #5 在本層（啟動時），**#4 不在本層**——per-seller
// 憑證是 runtime 從 DB 讀出來的，那是 guardEnvironment 的第二層，落點在
// internal/client/amego。本檔不假裝涵蓋它。

// baseValid 回傳一份「其餘欄位都合法」的 Config，讓每個 case 只變動它要測的那一格。
// 不從環境變數走 Load()：那會讓測試依賴 process 級的 env 狀態，平行跑時互相污染。
func baseValid() *Config {
	c := &Config{Env: "dev"}
	c.Server.HTTPPort = "10040"
	c.Server.GRPCPort = "10041"
	c.Server.FilePort = "10042"
	c.Database.Host = "postgresql"
	c.Database.User = "dbadmin"
	c.Database.Password = "pw"
	c.Database.DBName = "hawkr_invoice"
	c.Encryption.MasterKey = "master-key-32-bytes-for-unit-test"
	c.Encryption.MasterKeyVersion = 1
	c.Encryption.LookupHMACKey = "lookup-hmac-key-32-bytes-for-test"
	c.Amego.BaseURL = "https://invoice-api.amego.tw"
	c.Amego.TestInvoice = "12345678"
	c.Amego.PlatformInvoice = "12345678"
	c.Amego.AllowRealInvoice = false
	c.Amego.TimeoutIssueMS = 30000
	c.Amego.TimeoutQueryMS = 10000
	c.Schedule.IssueLeaseSeconds = 120
	c.Schedule.RecheckLeaseSeconds = 60
	c.Schedule.BackoffBaseSeconds = 30
	c.Schedule.BackoffMaxSeconds = 1800
	return c
}

// 前提檢查：baseValid 本身必須通過。少了這一支，下面每一條「期望轉紅」都可能是
// 因為別的欄位不合法而紅的——那種綠燈完全沒有鑑別力（同 §13.4 ②類的「注入真問題
// 後仍轉紅」要求）。
func TestBaseValidConfigPasses(t *testing.T) {
	if err := baseValid().Validate(); err != nil {
		t.Fatalf("baseValid() 應該通過，卻回 %v", err)
	}
}

// §4.5.8 驗收 #1：ALLOW_REAL=false ＋ 正式統編 ⇒ 對真實公司開出無法回收的真發票。
func TestValidate_RejectsRealTaxIDWhenRealInvoiceDisallowed(t *testing.T) {
	c := baseValid()
	c.Amego.AllowRealInvoice = false
	c.Amego.PlatformInvoice = "45444444" // 非測試統編
	err := c.Validate()
	if err == nil {
		t.Fatal("ALLOW_REAL=false 配正式統編必須被擋下")
	}
	if !strings.Contains(err.Error(), "AMEGO_PLATFORM_INVOICE") {
		t.Fatalf("錯誤訊息要指名是哪個變數，實際：%v", err)
	}
}

// §4.5.8 驗收 #2：ALLOW_REAL=true ＋ 測試統編 ⇒ 開出的發票不進任何公司的帳。
func TestValidate_RejectsTestTaxIDWhenRealInvoiceAllowed(t *testing.T) {
	c := baseValid()
	c.Amego.AllowRealInvoice = true
	c.Amego.PlatformInvoice = "12345678" // ＝ TestInvoice
	err := c.Validate()
	if err == nil {
		t.Fatal("ALLOW_REAL=true 配測試統編必須被擋下")
	}
	if !strings.Contains(err.Error(), "不會進入任何公司的帳") {
		t.Fatalf("錯誤訊息要說出後果，實際：%v", err)
	}
}

// §4.5.8 驗收 #3：反向失效。prod 沒開 ALLOW_REAL ⇒ 一張發票都開不出來，
// 而且沒有任何錯誤訊號——「開不出來」比「開錯」更容易被當成沒有訂單。
func TestValidate_RejectsProdWithoutAllowRealInvoice(t *testing.T) {
	for _, env := range []string{"prod", "production", "PROD", " production "} {
		t.Run(env, func(t *testing.T) {
			c := baseValid()
			c.Env = env
			c.Amego.AllowRealInvoice = false
			c.Amego.PlatformInvoice = "12345678"
			if err := c.Validate(); err == nil {
				t.Fatalf("ENV=%q 且 ALLOW_REAL=false 必須被擋下", env)
			}
		})
	}
}

// 判定依據必須是 AMEGO_ALLOW_REAL_INVOICE，**不是 Env == "prod"**（§4.6 W-21）。
// 反向斷言：非 prod 環境開了 ALLOW_REAL 且配正式統編，是合法的（例如 staging 對
// 真實統編做一次驗證）——把判準綁在 Env 上會誤擋這一格。
func TestValidate_AllowRealIsTheGateNotEnvName(t *testing.T) {
	c := baseValid()
	c.Env = "dev"
	c.Amego.AllowRealInvoice = true
	c.Amego.PlatformInvoice = "45444444"
	if err := c.Validate(); err != nil {
		t.Fatalf("判準應該是 ALLOW_REAL 而不是 Env，實際被擋：%v", err)
	}
}

// §4.5.8 驗收 #5：這一條驗的是「上面四條本身有沒有鑑別力」。
// AMEGO_TEST_INVOICE 是比較的基準；把它換成別的值，#1／#2 的判定必須跟著翻轉。
// 若換了基準四條全都不動，代表比較根本沒發生（例如兩邊都被當成空字串跳過）。
func TestValidate_TestInvoiceIsActuallyTheBaseline(t *testing.T) {
	c := baseValid()
	c.Amego.TestInvoice = "99999999" // 換基準
	c.Amego.AllowRealInvoice = false
	c.Amego.PlatformInvoice = "12345678" // 原本合法，現在不再等於基準
	if err := c.Validate(); err == nil {
		t.Fatal("換掉 AMEGO_TEST_INVOICE 之後 #1 必須翻轉成紅；沒翻轉代表比較沒發生")
	}

	c2 := baseValid()
	c2.Amego.TestInvoice = "99999999"
	c2.Amego.AllowRealInvoice = false
	c2.Amego.PlatformInvoice = "99999999" // 跟著新基準走 ⇒ 應該合法
	if err := c2.Validate(); err != nil {
		t.Fatalf("跟著新基準的設定應該合法，實際：%v", err)
	}
}

// 兩個統編都是必填。空字串會讓上面三條互斥斷言整段跳過（`!= ""` 的守衛），
// 那正是「斷言存在但從不執行」的形狀 —— 所以缺值必須在更前面就被擋下。
func TestValidate_TaxIDsAreRequired(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"缺 TEST_INVOICE", func(c *Config) { c.Amego.TestInvoice = "" }, "AMEGO_TEST_INVOICE"},
		// AMEGO_PLATFORM_INVOICE 自 #3230 起是**選填**，不在本表。
		// 它為空時必須通過 —— 由 TestValidate_PlatformInvoiceOptional_ProdCombination 守。
		// ⚠️ AMEGO_TEST_INVOICE 仍是必填且**不可順手一起放寬**：它是下面兩條
		// conflict 斷言的比對基準，拿掉它等於讓兩條的前提永遠不成立、守衛靜默失效。
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := baseValid()
			tc.mut(c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("%s 必須被擋下", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("訊息要指名 %s，實際：%v", tc.want, err)
			}
		})
	}
}

// 加密金鑰：lookup key 與 master key 必須不同（§8.1.1(f)）。
// 共用一把的後果不是「比較不安全」而是操作性的：master key 可隨時輪替，
// 而 lookup key 一旦輪替全表 fingerprint 同時失效。
// #3230：AMEGO_PLATFORM_INVOICE 為空時，**prod 的實際組合**必須能通過 Validate。
//
// 🔴 這一格只有單元測試驗得到。dev 的部署組合是 ENV=dev ＋ AllowReal=false ＋
// PlatformInvoice==TestInvoice，走不到「ENV=prod 才觸發」那條斷言；而 prod 唯一會走的
// 組合（ENV=prod ＋ AllowReal=true ＋ 平台統編空）在 dev 上根本不存在。
// 把驗證全押在 dev 環境 ＝ 漏測 prod 真正會走的那條分支。
//
// 這一格若失守，症狀就是 #3230：pod 起不來 → 部署者設 replicas: 0 → 76 條發票路由全 503。
func TestValidate_PlatformInvoiceOptional_ProdCombination(t *testing.T) {
	c := baseValid()
	c.Env = "prod"
	c.Amego.AllowRealInvoice = true
	c.Amego.PlatformInvoice = "" // 平台 issuer 尚未開通 —— 合法狀態

	if err := c.Validate(); err != nil {
		t.Fatalf("prod 的實際組合（ENV=prod ＋ AllowReal=true ＋ 平台統編為空）必須通過，實際被擋下：%v", err)
	}
}

// 平台統編為空**不得**讓 fail-closed 鬆一個位元：兩條 conflict 斷言的前提是
// 「TestInvoice 與 PlatformInvoice 皆非空」，一旦有人填了值，兩個災難方向必須照樣被擋。
//
// （兩個方向各自的正例在 TestValidate_RejectsRealTaxIDWhenRealInvoiceDisallowed 與
// TestValidate_RejectsTestTaxIDWhenRealInvoiceAllowed；本測試驗的是「放寬必填之後
// 那兩條仍然生效」，而不是重複它們。）
func TestValidate_OptionalPlatformInvoiceDoesNotWeakenGuards(t *testing.T) {
	// 空值 -> 通過（上一個測試已驗），但只要填了正式統編而未開放真發票，仍必須被擋
	c := baseValid()
	c.Amego.PlatformInvoice = "45444444"
	c.Amego.AllowRealInvoice = false
	if err := c.Validate(); err == nil {
		t.Fatal("放寬必填後，方向 A（未開放真發票卻用正式統編）仍必須被擋下")
	}

	// 反向：開放真發票卻用測試統編，同樣必須被擋
	c = baseValid()
	c.Env = "prod"
	c.Amego.PlatformInvoice = "12345678" // == TestInvoice
	c.Amego.AllowRealInvoice = true
	if err := c.Validate(); err == nil {
		t.Fatal("放寬必填後，方向 B（開放真發票卻用測試統編）仍必須被擋下")
	}
}

func TestValidate_LookupKeyMustDifferFromMasterKey(t *testing.T) {
	c := baseValid()
	c.Encryption.LookupHMACKey = c.Encryption.MasterKey
	if err := c.Validate(); err == nil {
		t.Fatal("兩把金鑰相同必須被擋下")
	}
}

// 租約下界（§4.3.3）：租約必須大於單筆最壞耗時，否則持有者還在打 AMEGO
// 就被另一個副本合法搶走 ⇒ 重複開票。編譯、測試、/metrics 全部正常。
func TestValidate_LeaseMustExceedHTTPTimeout(t *testing.T) {
	c := baseValid()
	c.Schedule.IssueLeaseSeconds = 20 // 20s < 30000ms
	err := c.Validate()
	if err == nil {
		t.Fatal("租約小於 issue timeout 必須被擋下")
	}
	if !strings.Contains(err.Error(), "INVOICE_ISSUE_LEASE_SECONDS") {
		t.Fatalf("訊息要指名變數，實際：%v", err)
	}
}

// credential_key_version 是 smallint（§5.1.3）。上界寫成 99999 的話，落在
// (32767, 99999] 的值 boot 期會通過，卻在**寫入第一列憑證時**才撞 22003——
// 那時錯誤出現在一個與設定無關的地方，而 pod 早就 Ready 了。
func TestValidate_MasterKeyVersionRespectsSmallintUpperBound(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version int
		wantErr bool
	}{
		{"下界 1 合法", 1, false},
		{"smallint 上限 32767 合法", 32767, false},
		{"32768 超過 smallint", 32768, true},
		{"舊上界 99999 必須被擋", 99999, true},
		{"0 不合法", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := baseValid()
			c.Encryption.MasterKeyVersion = tc.version
			err := c.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("version=%d 必須被擋下", tc.version)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("version=%d 應該合法，實際：%v", tc.version, err)
			}
		})
	}
}
