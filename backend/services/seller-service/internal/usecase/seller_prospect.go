package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// ProspectMatchTier 是一次比對命中的信心層級。
//
// 刻意只有兩層：T3（名稱模糊相似）永不在此自動合併，它屬於 admin 建檔
// 表單上的「可能重複」提示（spec D6）。把它放進這裡就等於讓一支背景程式
// 自行決定兩家業者是同一家——合併不可逆，代價遠高於漏合併。
type ProspectMatchTier string

const (
	ProspectMatchTierTaxID   ProspectMatchTier = "T1_TAX_ID"
	ProspectMatchTierContact ProspectMatchTier = "T2_CONTACT"
)

// ProspectMatchCandidate 是一筆比對結果。MatchedOn 讓呼叫端（遷移報告、
// admin 表單提示）能對人說明「為什麼認為是同一家」，不是只給一個布林。
type ProspectMatchCandidate struct {
	Prospect  domain.SellerProspect
	Tier      ProspectMatchTier
	MatchedOn string // tax_id | email | phone | domain | social
}

type SellerProspectUseCase struct {
	repo domain.SellerProspectRepository
	// mediaClient 只給品牌圖片那一條路徑用（見 seller_prospect_media.go）。
	// 與 SellerUseCase 共用同一個 MediaClient 介面，不另開第二套 client。
	mediaClient MediaClient
}

func NewSellerProspectUseCase(repo domain.SellerProspectRepository, mediaClient MediaClient) *SellerProspectUseCase {
	return &SellerProspectUseCase{repo: repo, mediaClient: mediaClient}
}

// ErrProspectNotConvertible 表示這筆 prospect 不處於可歸因狀態——已入駐
// （CONVERTED）、已封存，或已被合併進另一列。三種情況都由同一個述詞
// （status = 'ACTIVE'）擋下，rows = 0 是唯一的訊號。
var ErrProspectNotConvertible = errors.New("prospect is not in a convertible state")

// ErrProspectTaxIDConflict／ErrProspectConvertedStoreConflict 是 23505 的具名
// 轉譯：告訴操作者「哪個欄位」撞了唯一性，不是把 DB constraint name
// （uq_seller_prospects_tax_id／uq_seller_prospects_converted_store）原樣丟
// 出去——那不是給人看的介面，且在 production 會被 ErrorInterceptor 的預設
// Internal 遮蔽路徑吃掉，操作者只會看到「系統錯誤」而不知道自己撞了什麼。
//
// 兩者都由 translateProspectDuplicate 從 repository 回傳的 *domain.ErrDuplicate
// 轉出，涵蓋 Create／UpdateForAdmin／Merge／markProspectConverted 四個寫入點
// （見各自呼叫處）。
var (
	ErrProspectTaxIDConflict          = errors.New("此統一編號已被其他業者使用中")
	ErrProspectConvertedStoreConflict = errors.New("此 store 已被其他站外業者標記為已入駐")
)

// translateProspectDuplicate 把 repository 對 seller_prospects 唯一性違反
// （*domain.ErrDuplicate）轉成上面兩個具名 sentinel 之一，讓 handler 只需要
// errors.Is 判斷、不必知道 DB constraint 長什麼樣子。err 不是 *domain.ErrDuplicate
// 或 constraint name 不在已知清單內（理論上不該出現，未來若替這張表加新的
// 唯一約束卻忘記在這裡登記）時原樣傳回，交給既有的 Internal fallback 處理
// ——寧可退回舊行為（洩漏一次未登記的 constraint name），也不要對一個
// 認不出來的錯誤瞎猜欄位名字騙操作者。
func translateProspectDuplicate(err error) error {
	var dup *domain.ErrDuplicate
	if !errors.As(err, &dup) {
		return err
	}
	switch dup.ConstraintName {
	case "uq_seller_prospects_tax_id":
		return ErrProspectTaxIDConflict
	case "uq_seller_prospects_converted_store":
		return ErrProspectConvertedStoreConflict
	default:
		return err
	}
}

// markProspectConverted 是「rows == 0 代表這筆不可歸因」這條轉譯的唯一定義。
// CreateSeller 的 token 路徑（attributeProspect，直接傳
// prospectRepo.MarkConverted）與 admin 的手動綁定（經由
// SellerProspectUseCase.MarkConverted，傳 uc.repo.MarkConverted）都經過
// 這裡——兩邊各寫一次 rows == 0 的判斷，就是兩份會分岔的二次轉換防護。
//
// operatorAccountID 原樣轉交給 mark：admin 手動綁定傳操作者的登入
// principal，CreateSeller 的 token 路徑傳 nil（見 attributeProspect 的呼叫
// 處）——這一欄只記「這次歸因是誰做的」，不影響這裡是單一寫入點的事實。
//
// 吃一個 func 而不是 domain.SellerProspectRepository：呼叫端只需要
// MarkConverted 這一支方法，沒有理由要求它們持有或組裝一個完整的
// repository（見 attributeProspect 使用窄介面 ProspectAttributionRepo 的
// 理由），也不需要靠嵌入 nil 介面去湊過編譯檢查。
func markProspectConverted(ctx context.Context, mark func(context.Context, int64, int64, *int64) (int64, error), prospectID, storeID int64, operatorAccountID *int64) error {
	rows, err := mark(ctx, prospectID, storeID, operatorAccountID)
	if err != nil {
		return translateProspectDuplicate(err)
	}
	if rows == 0 {
		return ErrProspectNotConvertible
	}
	return nil
}

// MarkConverted 是歸因的唯一寫入點（給 admin 手動綁定用）。CreateSeller 的
// token 路徑與這裡共用同一份轉譯邏輯，見 markProspectConverted。
func (uc *SellerProspectUseCase) MarkConverted(ctx context.Context, prospectID, storeID int64, operatorAccountID *int64) error {
	return markProspectConverted(ctx, uc.repo.MarkConverted, prospectID, storeID, operatorAccountID)
}

// ErrStoreNotFound 表示要綁定的 store 不存在或不是 current stint。
var ErrStoreNotFound = errors.New("store not found or not current")

// ConvertForAdmin 是 spec §5.2 的第二、三條歸因路徑。寫入走與 CreateSeller
// 完全相同的 MarkConverted——三條路都走同一個寫入點，二次轉換防護才只需要
// 維護一份。operatorAccountID 是這次手動歸因的操作者登入 principal
// （spec §5.3／§6 要求記錄），落地到 updated_by_account_id。
func (uc *SellerProspectUseCase) ConvertForAdmin(ctx context.Context, prospectID, storeID int64, operatorAccountID *int64) error {
	exists, err := uc.repo.CurrentStoreExists(ctx, storeID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrStoreNotFound
	}
	return uc.MarkConverted(ctx, prospectID, storeID, operatorAccountID)
}

// ListTaxIDMatches 是 spec §5.2 第三條歸因路徑的清單頁：統編對得上、但從未
// 被連結的 prospect／已入駐賣家配對。
func (uc *SellerProspectUseCase) ListTaxIDMatches(ctx context.Context) ([]domain.ProspectTaxIDMatch, error) {
	return uc.repo.ListTaxIDMatches(ctx)
}

// Match 是識別解析的唯一進入點，遷移與 admin 建檔共用（spec D5）。
//
// T1 命中即短路：統編是權威識別碼，比中了就沒有再看聯絡方式的必要，
// 而且繼續往下查只會製造一堆低信心的雜訊候選給人審。
func (uc *SellerProspectUseCase) Match(ctx context.Context, in domain.ProspectContactInput) ([]ProspectMatchCandidate, error) {
	keys := domain.BuildProspectMatchKeys(in)

	if keys.TaxID != "" {
		p, err := uc.repo.FindByTaxID(ctx, keys.TaxID)
		switch {
		case err == nil:
			return []ProspectMatchCandidate{{
				Prospect: p, Tier: ProspectMatchTierTaxID, MatchedOn: "tax_id",
			}}, nil
		case errors.Is(err, pgx.ErrNoRows):
			// 沒這個統編，往下走 T2。
		default:
			return nil, err
		}
	}

	// 四個聯絡鍵全空時不查：真實 postgres repo 內部也擋這一次（見
	// internal/repository/postgres/seller_prospect.go FindByMatchKeys），
	// 但這一層仍要自己擋一次——呼叫端可能是任何 domain.SellerProspectRepository
	// 實作，不能假設每個實作都記得這個不變式；且「有鍵才查」在這裡看得見。
	if keys.Email == "" && keys.Phone == "" && keys.Domain == "" && keys.Social == "" {
		return nil, nil
	}

	rows, err := uc.repo.FindByMatchKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	out := make([]ProspectMatchCandidate, 0, len(rows))
	for _, p := range rows {
		out = append(out, ProspectMatchCandidate{
			Prospect: p, Tier: ProspectMatchTierContact, MatchedOn: matchedOn(keys, p.MatchKeys),
		})
	}
	return out, nil
}

// matchedOn 回報是哪一個鍵讓這兩列被判為同一家。順序即優先序：
// email 最具識別力，social 次之，phone 與 domain 最容易被共用。
//
// 必須比對 got（回傳列自己的鍵），不能只看 want（查詢本身用了哪個鍵）：
// FindByMatchKeys 是 OR 查詢，查詢同時帶 email 與 phone 時，回傳列可能只
// 因 phone 相同而命中、email 完全不同。這裡的字串要進 admin 提示與遷移
// 報告，對人解釋「為什麼認為是同一家」——回報一個沒有真的相同的鍵等於
// 對人謊報證據，而合併不可逆，假證據比沒有證據更糟。
func matchedOn(want, got domain.ProspectMatchKeys) string {
	switch {
	case want.Email != "" && want.Email == got.Email:
		return "email"
	case want.Social != "" && want.Social == got.Social:
		return "social"
	case want.Phone != "" && want.Phone == got.Phone:
		return "phone"
	case want.Domain != "" && want.Domain == got.Domain:
		return "domain"
	}
	return ""
}

func (uc *SellerProspectUseCase) Search(ctx context.Context, params domain.ProspectSearchParams) ([]domain.ProspectSearchHit, error) {
	return uc.repo.Search(ctx, params)
}

var (
	prospectVendorTypes = map[string]struct{}{"individual": {}, "business": {}}
	prospectStatuses    = map[string]struct{}{"ACTIVE": {}, "ARCHIVED": {}, "CONVERTED": {}}
	prospectOutreach    = map[string]struct{}{
		"NOT_CONTACTED": {}, "INVITED": {}, "REPLIED": {},
		"DECLINED": {}, "DO_NOT_CONTACT": {}, "CONVERTED": {},
	}
)

// ValidateProspectFields 把 seller_prospects 的四條 CHECK 約束在 usecase 先擋一次。
//
// 為何四條一起而不是只擋這次碰到的那條：b99d7785b 只擋了 tax_id，因為那是當時
// dev 上實測炸出來的一條。CHECK 約束變成 500 是一整類問題，不是一個實例——
// admin 更新表單會同時餵 vendor_type／status／outreach_status，任一條漏掉就是
// 「使用者選錯一個值、畫面回 500」，而且把 DB 約束名洩漏到 service 邊界外。
func ValidateProspectFields(in domain.SellerProspect) error {
	verr := &ValidationError{}
	if _, ok := prospectVendorTypes[in.VendorType]; !ok {
		verr.add("業者類型不合法")
	}
	if _, ok := prospectStatuses[in.Status]; !ok {
		verr.add("狀態不合法")
	}
	if _, ok := prospectOutreach[in.OutreachStatus]; !ok {
		verr.add("招商狀態不合法")
	}
	if in.TaxID != nil && !banFormatRe.MatchString(*in.TaxID) {
		verr.add("統一編號格式不符")
	}
	if verr.HasErrors() {
		return verr
	}
	return nil
}

// Create 建立一筆站外業者。比對不在這裡做——呼叫端（handler）必須先
// 呼叫 Match、把結果交給人看過，才決定要不要走到這一步。把比對藏進
// Create 裡會讓「自動合併」變成預設行為，那正是 spec D6 禁止的事。
//
// 統編格式在這裡先擋一次：不擋的話唯一的防線是 DB 的 chk_sp_tax_id，
// 呼叫端拿到的是 `Internal: ... violates check constraint "chk_sp_tax_id"
// (SQLSTATE 23514)`——2026-09-06 dev 實測到的真實回應。那對階段二的 admin
// 表單而言是「使用者打錯一個字，畫面回 500」，而且把 DB 約束名洩漏到
// service 邊界外。沿用同套件既有的 banFormatRe（統編就是統編，不另立
// 第二套規則），*ValidationError 由既有的 ErrorInterceptor 映射成
// InvalidArgument。
func (uc *SellerProspectUseCase) Create(ctx context.Context, in domain.SellerProspect) (domain.SellerProspect, error) {
	if err := ValidateProspectFields(in); err != nil {
		return domain.SellerProspect{}, err
	}
	created, err := uc.repo.Create(ctx, in)
	if err != nil {
		return domain.SellerProspect{}, translateProspectDuplicate(err)
	}
	return created, nil
}

func (uc *SellerProspectUseCase) ListForAdmin(ctx context.Context, params domain.ProspectAdminListParams) ([]domain.SellerProspect, int64, error) {
	return uc.repo.ListForAdmin(ctx, params)
}

// GetForAdmin 一併回邀請歷史：admin 詳情頁的邀請面板與主體是同一次載入，
// 分兩支 RPC 只會讓前端多一次往返而沒有任何取捨可言。
func (uc *SellerProspectUseCase) GetForAdmin(ctx context.Context, id int64) (domain.SellerProspect, []domain.ProspectInvite, error) {
	p, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return domain.SellerProspect{}, nil, err
	}
	invites, err := uc.repo.ListInvites(ctx, id)
	if err != nil {
		return domain.SellerProspect{}, nil, err
	}
	return p, invites, nil
}

// ErrProspectMerged 表示這筆列已被合併進另一列，不可還原也不可再被操作。
var ErrProspectMerged = errors.New("prospect has been merged into another row")

// UpdateForAdmin 在寫入前重算 match keys，並跑與 Create 同一套欄位驗證。
func (uc *SellerProspectUseCase) UpdateForAdmin(ctx context.Context, in domain.SellerProspect) (domain.SellerProspect, error) {
	in.Status = "ACTIVE" // Update 的述詞只接受 ACTIVE；驗證用得到這個值
	// outreach_status = 'CONVERTED' 是 markProspectConverted（唯一寫入點，見
	// 該函式註解）的既成事實，不是 admin 表單可以手動選的值：放行的話會
	// 產生「outreach_status 說已入駐，但 status 還是 ACTIVE、
	// converted_store_id 是 NULL」的矛盾列——SearchSellerProspects 只濾
	// status，這種列會繼續出現在 EXTERNAL lane 餵給買家，對外宣稱一個從未
	// 真的發生過的入駐（spec §2.1）。prospectOutreach 仍然收 CONVERTED——
	// 那是轉換路徑寫入後這一欄合法的既有值，仍要能通過 Create／既有列的
	// 一般驗證；這裡擋的是「admin 選它」這個動作本身，不是這個值本身不合法。
	if in.OutreachStatus == "CONVERTED" {
		return domain.SellerProspect{}, &ValidationError{Messages: []string{"招商狀態不可手動設為 CONVERTED，此狀態只能由入駐流程寫入"}}
	}
	if err := ValidateProspectFields(in); err != nil {
		return domain.SellerProspect{}, err
	}
	in.MatchKeys = domain.BuildProspectMatchKeys(domain.ProspectContactInput{
		TaxID:   derefOrEmpty(in.TaxID),
		Email:   in.PublicEmail,
		Phone:   in.PublicPhone,
		Website: in.PublicWebsite,
		Social:  in.PublicSocial,
	})
	updated, err := uc.repo.Update(ctx, in)
	if err != nil {
		return domain.SellerProspect{}, translateProspectDuplicate(err)
	}
	return updated, nil
}

// SetArchived 封存或還原一筆 prospect。rows = 0 代表 SQL 述詞沒有放行
// （封存要求目前是 ACTIVE；還原額外要求 merged_into_prospect_id 為
// NULL——被合併的列不可還原，本輪不提供合併的反向操作），一律轉成
// ErrProspectMerged。
//
// 還原把列改回 ACTIVE，可能撞上 uq_seller_prospects_tax_id（封存期間別的
// 業者合法拿走了同一個統編，見 repository.SetArchived 的註解）——同
// Create／UpdateForAdmin，這裡也要走 translateProspectDuplicate，讓
// handler 能用同一份 prospectConflictStatus 判斷，不落到 Internal 遮蔽。
func (uc *SellerProspectUseCase) SetArchived(ctx context.Context, id int64, archived bool, operatorID *int64) error {
	rows, err := uc.repo.SetArchived(ctx, id, archived, operatorID)
	if err != nil {
		return translateProspectDuplicate(err)
	}
	if rows == 0 {
		return ErrProspectMerged
	}
	return nil
}

// ProspectDuplicatePair 是一對疑似重複 ＋ 是哪個鍵讓它們被判為同一家。
type ProspectDuplicatePair struct {
	Left      domain.SellerProspect
	Right     domain.SellerProspect
	MatchedOn string
}

const prospectDuplicateLimit = 500

// ListDuplicates 掃出全表的 T2 成對候選。第二個回傳值為 true 表示還有沒回的。
//
// MatchedOn 由既有的 matchedOn() 計算而不是在 SQL 裡再寫一次優先序：
// 那份優先序有自己的理由（5979c04bc：必須比對回傳列的真實鍵），不能有第二份。
// matchedOn 的每個分支都帶 want.X != "" 守衛，因此對 (a, b) 與 (b, a) 對稱。
func (uc *SellerProspectUseCase) ListDuplicates(ctx context.Context) ([]ProspectDuplicatePair, bool, error) {
	raw, err := uc.repo.ListDuplicatePairs(ctx)
	if err != nil {
		return nil, false, err
	}
	truncated := len(raw) > prospectDuplicateLimit
	if truncated {
		raw = raw[:prospectDuplicateLimit]
	}
	pairs := make([]ProspectDuplicatePair, 0, len(raw))
	for _, p := range raw {
		pairs = append(pairs, ProspectDuplicatePair{
			Left:      p.Left,
			Right:     p.Right,
			MatchedOn: matchedOn(p.Left.MatchKeys, p.Right.MatchKeys),
		})
	}
	return pairs, truncated, nil
}

// ErrProspectMergeNotAllowed re-exports domain.ErrProspectMergeNotAllowed so
// gRPC handler code (Task 12) can write errors.Is(err, usecase.ErrProspectMergeNotAllowed)
// alongside the sibling ErrProspectNotConvertible／ErrProspectMerged above,
// without callers needing to know it actually originates one layer down.
//
// The value itself has to live in domain, not here: internal/usecase already
// imports internal/repository/postgres (see internal/usecase/di.go), so the
// repository cannot import this package back to reference a usecase-level
// sentinel without creating an import cycle — domain is the lowest layer
// both sides can reach, same as domain.ErrNoActiveSellerMembership.
var ErrProspectMergeNotAllowed = domain.ErrProspectMergeNotAllowed

// Merge 是薄層：真正的鎖定順序、狀態斷言、搬邀請、路徑壓縮與封存都在
// repository 的單一交易內完成（domain.SellerProspectRepository.Merge 的
// 文件註解），這裡不重複那份邏輯。四種拒絕情境都由 repository 包成
// ErrProspectMergeNotAllowed，這裡原樣往上傳，不再轉譯一次。
func (uc *SellerProspectUseCase) Merge(ctx context.Context, winnerID, loserID int64, operatorID *int64) (domain.SellerProspect, error) {
	winner, err := uc.repo.Merge(ctx, winnerID, loserID, operatorID)
	if err != nil {
		return domain.SellerProspect{}, translateProspectDuplicate(err)
	}
	return winner, nil
}

const (
	prospectInviteTokenBytes   = 32
	prospectInviteDefaultDays  = 30
	prospectInviteMaxValidDays = 180
)

// ErrProspectNotInvitable 表示這筆 prospect 目前不是 ACTIVE，不能建邀請——
// 已入駐（CONVERTED）或已封存／被合併（ARCHIVED）都落在這裡，同
// ErrProspectMerged／ErrProspectNotConvertible 的既有慣例：SQL 述詞只給
// 一個布林訊號，不細分是哪一種成因。
var ErrProspectNotInvitable = errors.New("prospect is not in an invitable state")

// CreateInvite 產生一筆邀請。token 是 CSPRNG 的不透明隨機值而非簽章——
// 簽章 token 無法撤銷，而邀請連結需要撤銷、到期、單次消費與點擊軌跡
// (spec §3.2)。它只做歸因，絕不授予任何權限 (spec §5.3)。
//
// 明文 token 只在這裡回一次，之後任何列表都不再回傳。
func (uc *SellerProspectUseCase) CreateInvite(ctx context.Context, prospectID int64, validDays int) (domain.ProspectInvite, string, error) {
	p, err := uc.repo.GetByID(ctx, prospectID)
	if err != nil {
		return domain.ProspectInvite{}, "", err
	}
	// 只有 ACTIVE 的 prospect 可以建邀請。GetByID 沒有 status 述詞（見
	// domain.SellerProspectRepository.GetByID 的介面註解），一筆已被合併
	// 掉／已入駐的 prospect 一樣查得到——不擋的話，合併後仍在 ARCHIVED 狀態
	// 可見的 loser 詳情頁能繼續建邀請、發連結，vendor 用它註冊時
	// MarkConverted 的 WHERE status = 'ACTIVE' 影響 0 列、歸因悄悄失敗
	// （CreateSeller 只記 log warning、不擋建店），token 卻已消費，整段合併
	// 想擋的事又重新發生一次（merge 第 3 步：搬邀請到 winner，就是為了不讓
	// 邀請繼續指向一筆會被封存的列）。
	if p.Status != "ACTIVE" {
		return domain.ProspectInvite{}, "", ErrProspectNotInvitable
	}
	if validDays <= 0 {
		validDays = prospectInviteDefaultDays
	}
	if validDays > prospectInviteMaxValidDays {
		validDays = prospectInviteMaxValidDays
	}

	buf := make([]byte, prospectInviteTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return domain.ProspectInvite{}, "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)

	invite, err := uc.repo.CreateInvite(ctx, prospectID, token, time.Now().Add(time.Duration(validDays)*24*time.Hour))
	if err != nil {
		return domain.ProspectInvite{}, "", err
	}
	return invite, token, nil
}

// ErrProspectInviteNotRevocable 表示這個 (prospectID, inviteID) 組合查無
// 可撤銷的邀請——id 不存在、prospect_id 對不上、已撤銷，或已消費。已消費
// 的邀請不再可撤銷：那筆歸因已經發生，撤銷它只會讓歷史說謊。
var ErrProspectInviteNotRevocable = errors.New("invite not found, already revoked, or already consumed")

// RevokeInvite 撤銷一筆邀請。同時比對 prospectID 與 inviteID 是 repository
// 層 SQL 的既有述詞（見 RevokeProspectInvite），這裡只把 rows == 0 轉成
// 具名錯誤，不重複那份比對邏輯。
func (uc *SellerProspectUseCase) RevokeInvite(ctx context.Context, prospectID, inviteID int64) error {
	rows, err := uc.repo.RevokeInvite(ctx, prospectID, inviteID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrProspectInviteNotRevocable
	}
	return nil
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
