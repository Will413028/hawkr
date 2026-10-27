package grpc

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

// prospectAdminUseCase 是 admin console（階段二）用的窄介面，跟既有
// prospectSearcher／prospectCreator 同風格：刻意窄化，讓 handler 測試不必
// 組出整個 usecase 與其 repository 依賴。Task 12 會再擴充這個介面（重複、
// 合併、歸因、對帳），欄位與 handler.go 的接線不需要再動。
type prospectAdminUseCase interface {
	ListForAdmin(ctx context.Context, params domain.ProspectAdminListParams) ([]domain.SellerProspect, int64, error)
	GetForAdmin(ctx context.Context, id int64) (domain.SellerProspect, []domain.ProspectInvite, error)
	UpdateForAdmin(ctx context.Context, in domain.SellerProspect) (domain.SellerProspect, error)
	SetArchived(ctx context.Context, id int64, archived bool, operatorID *int64) error
	CreateInvite(ctx context.Context, prospectID int64, validDays int) (domain.ProspectInvite, string, error)
	RevokeInvite(ctx context.Context, prospectID, inviteID int64) error
	ListDuplicates(ctx context.Context) ([]usecase.ProspectDuplicatePair, bool, error)
	Merge(ctx context.Context, winnerID, loserID int64, operatorID *int64) (domain.SellerProspect, error)
	ConvertForAdmin(ctx context.Context, prospectID, storeID int64, operatorID *int64) error
	ListTaxIDMatches(ctx context.Context) ([]domain.ProspectTaxIDMatch, error)
	UpdateProspectMedia(ctx context.Context, in usecase.ProspectMediaInput) (domain.SellerProspect, error)
}

const (
	prospectAdminDefaultPage     = 1
	prospectAdminDefaultPageSize = 20
	prospectAdminMaxPageSize     = 500
)

// parseProspectAdminID 把 wire 上的字串 id 解析成 int64，解析失敗回
// *usecase.ValidationError——由既有 ErrorInterceptor 映射成 InvalidArgument。
// 跟 seller_prospect_handler.go 的 atoiOrZero 刻意不同：那裡的星等只是排序
// 次要訊號，格式錯誤容錯成 0 才是對的；這裡的 id 是識別鍵，格式錯誤代表
// 呼叫端送錯了東西，必須讓它知道，不能悄悄查一個不存在的列。
func parseProspectAdminID(raw, field string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, &usecase.ValidationError{Messages: []string{field + " 格式不正確"}}
	}
	return id, nil
}

// parseOperatorAccountID 解析操作者的登入 principal。空字串、非數字、或 0
// 一律視為「沒有」，回 nil——跟 seller_prospect_handler.go CreateSellerProspect
// 對 created_by_account_id／updated_by_account_id 的既有處置一致：這一欄是
// 稽核追蹤，不是識別鍵，格式問題不該讓操作失敗。
func parseOperatorAccountID(raw string) *int64 {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v == 0 {
		return nil
	}
	return &v
}

// optRFC3339 把可能為 nil 的時間戳格式化成 wire 上的字串；nil 一律回空字串。
func optRFC3339(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// prospectNotFoundStatus 把 pgx.ErrNoRows 轉成帶指定訊息的 NotFound
// status.Error；err 不是 pgx.ErrNoRows 時回 nil，由呼叫端決定怎麼處理其餘
// 錯誤——三個呼叫端（GetForAdmin、CreateInvite 內部的 GetByID 存在性檢查、
// UpdateForAdmin 的 `:one` 查詢 WHERE status = 'ACTIVE'）都會冒出同一個
// pgx.ErrNoRows，這裡收斂成一份判斷，不要求三處各自重寫一次 errors.Is。
//
// 不把「非 ErrNoRows 時要怎麼辦」也收進這支函式：UpdateForAdmin 的錯誤
// 面還包含 ValidateProspectFields 產生的 *usecase.ValidationError（一個
// DomainCoder，必須原樣傳給既有 ErrorInterceptor 映射成 InvalidArgument），
// 如果這裡連同「非 ErrNoRows 一律包成 Internal」都做了，會連帶把驗證錯誤
// 也吃成 Internal。
func prospectNotFoundStatus(err error, notFoundMsg string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Error(codes.NotFound, notFoundMsg)
	}
	return nil
}

// prospectConflictStatus 把 usecase 對 seller_prospects 唯一性違反的具名轉譯
// （usecase.ErrProspectTaxIDConflict／ErrProspectConvertedStoreConflict，見
// usecase/seller_prospect.go translateProspectDuplicate）轉成 AlreadyExists
// status，訊息點名是哪個欄位撞到唯一性，不是把 DB constraint name 丟給操作
// 者。err 不是這兩種之一時回 nil，由呼叫端決定其餘錯誤怎麼處理——同
// prospectNotFoundStatus 的既有慣例。五個寫入路徑共用同一份判斷：
// CreateSellerProspect（seller_prospect_handler.go）、AdminUpdateSellerProspect、
// AdminMergeSellerProspects、AdminConvertSellerProspect、
// AdminArchiveSellerProspect（還原時可能撞 uq_seller_prospects_tax_id）。
func prospectConflictStatus(err error) error {
	switch {
	case errors.Is(err, usecase.ErrProspectTaxIDConflict):
		return status.Error(codes.AlreadyExists, usecase.ErrProspectTaxIDConflict.Error())
	case errors.Is(err, usecase.ErrProspectConvertedStoreConflict):
		return status.Error(codes.AlreadyExists, usecase.ErrProspectConvertedStoreConflict.Error())
	}
	return nil
}

// toAdminProspectItemPB 是 admin console 的完整投影，與 toProspectItemPB／
// toProspectSearchItemPB 的差別是多了識別鍵與狀態欄位（status、tax_id、
// converted_*、merged_into_prospect_id、稽核欄位）——這些刻意不出現在
// ai-service 消費的 SellerProspectItem 上。可空欄位一律以空字串表達，
// 不是 "0" 或 "<nil>"；數字一律以十進位字串過線（Proto 管理規範 §6.1）。
func toAdminProspectItemPB(p domain.SellerProspect) *sellerpb.AdminSellerProspectItem {
	return &sellerpb.AdminSellerProspectItem{
		Id:                   formatID(p.ID),
		DisplayName:          p.DisplayName,
		VendorType:           p.VendorType,
		Status:               p.Status,
		OutreachStatus:       p.OutreachStatus,
		TaxId:                strDeref(p.TaxID),
		ConvertedStoreId:     optID(p.ConvertedStoreID),
		ConvertedAt:          optRFC3339(p.ConvertedAt),
		MergedIntoProspectId: optID(p.MergedIntoProspectID),
		Category:             p.Category,
		Region:               p.Region,
		ProductSummary:       p.ProductSummary,
		Positioning:          p.Positioning,
		ResearchSummary:      p.ResearchSummary,
		PriorityLetter:       p.PriorityLetter,
		PriorityStars:        strconv.Itoa(p.PriorityStars),
		PublicEmail:          p.PublicEmail,
		PublicPhone:          p.PublicPhone,
		PublicWebsite:        p.PublicWebsite,
		PublicSocial:         p.PublicSocial,
		CreatedByAccountId:   optID(p.CreatedByAccountID),
		UpdatedByAccountId:   optID(p.UpdatedByAccountID),
		CreatedAt:            p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:            p.UpdatedAt.UTC().Format(time.RFC3339),

		LogoKey:        p.LogoKey,
		CoverKey:       p.CoverKey,
		LogoSourceUrl:  p.LogoSourceURL,
		CoverSourceUrl: p.CoverSourceURL,
		MediaVersion:   strconv.FormatInt(p.MediaVersion, 10),
	}
}

// toProspectInvitePB 投影一筆邀請。sent_at／clicked_at／consumed_at／
// revoked_at 為 nil 時一律回空字串；expires_at／created_at 一筆邀請必然有值。
func toProspectInvitePB(i domain.ProspectInvite) *sellerpb.ProspectInviteItem {
	return &sellerpb.ProspectInviteItem{
		Id:         formatID(i.ID),
		ProspectId: formatID(i.ProspectID),
		ExpiresAt:  i.ExpiresAt.UTC().Format(time.RFC3339),
		SentAt:     optRFC3339(i.SentAt),
		ClickedAt:  optRFC3339(i.ClickedAt),
		ConsumedAt: optRFC3339(i.ConsumedAt),
		RevokedAt:  optRFC3339(i.RevokedAt),
		CreatedAt:  i.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// AdminListSellerProspects 是 admin 列表頁的查詢入口。page／page_size 兩個
// 預設值（1、20）與上限（500）都是 wire 契約的一部分：前端不送就是要
// 這個結果，上限收斂要在這一層做——只做在 repository 那一層（Task 5 Step 4）
// 的話，換一個呼叫端就繞過去了。
func (s *SellerServer) AdminListSellerProspects(ctx context.Context, req *sellerpb.AdminListSellerProspectsRequest) (*sellerpb.AdminListSellerProspectsResponse, error) {
	page := atoiOrZero(req.GetPage())
	if page < 1 {
		page = prospectAdminDefaultPage
	}
	pageSize := atoiOrZero(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = prospectAdminDefaultPageSize
	}
	if pageSize > prospectAdminMaxPageSize {
		pageSize = prospectAdminMaxPageSize
	}

	rows, total, err := s.prospectAdmin.ListForAdmin(ctx, domain.ProspectAdminListParams{
		Status:         req.GetStatus(),
		OutreachStatus: req.GetOutreachStatus(),
		VendorType:     req.GetVendorType(),
		Category:       req.GetCategory(),
		Region:         req.GetRegion(),
		Keyword:        req.GetKeyword(),
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		return nil, err
	}

	data := make([]*sellerpb.AdminSellerProspectItem, 0, len(rows))
	for _, row := range rows {
		data = append(data, toAdminProspectItemPB(row))
	}
	return &sellerpb.AdminListSellerProspectsResponse{
		Code:     0,
		Data:     data,
		Total:    strconv.FormatInt(total, 10),
		Page:     strconv.Itoa(page),
		PageSize: strconv.Itoa(pageSize),
	}, nil
}

// buildAdminProspectDetailResponse 呼叫 GetForAdmin 並映射成
// AdminGetSellerProspectResponse。AdminGetSellerProspect 與
// AdminArchiveSellerProspect（封存／還原後回傳最新狀態，讓 admin console
// 不必再補一次往返）共用同一段邏輯：三支 RPC（含 Task 12 的
// AdminConvertSellerProspect）都回同一個 proto message，Code: 0 就該代表
// Data 已經是最新狀態，不能有的回填、有的不回填。
//
// 查無資料時 GetForAdmin 原樣往上傳 pgx.ErrNoRows（見
// domain.SellerProspectRepository.GetByID 的介面註解），這裡用 errors.Is
// 辨識並轉成 NotFound；其餘沒被辨識的錯誤直接包成 Internal 而不是原樣往上
// 傳——本套件的 handler 測試直接呼叫 handler 函式，不會經過
// ErrorInterceptor 那層 unary interceptor，原樣傳回會讓 status.Code(err)
// 落在 Unknown 而不是 Internal。
func (s *SellerServer) buildAdminProspectDetailResponse(ctx context.Context, id int64) (*sellerpb.AdminGetSellerProspectResponse, error) {
	p, invites, err := s.prospectAdmin.GetForAdmin(ctx, id)
	if err != nil {
		if nf := prospectNotFoundStatus(err, "查無此站外業者"); nf != nil {
			return nil, nf
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	inviteItems := make([]*sellerpb.ProspectInviteItem, 0, len(invites))
	for _, inv := range invites {
		inviteItems = append(inviteItems, toProspectInvitePB(inv))
	}
	return &sellerpb.AdminGetSellerProspectResponse{
		Code: 0, Data: toAdminProspectItemPB(p), Invites: inviteItems,
	}, nil
}

// AdminGetSellerProspect 回傳單筆詳情連同邀請歷史。
func (s *SellerServer) AdminGetSellerProspect(ctx context.Context, req *sellerpb.AdminGetSellerProspectRequest) (*sellerpb.AdminGetSellerProspectResponse, error) {
	id, err := parseProspectAdminID(req.GetId(), "id")
	if err != nil {
		return nil, err
	}
	return s.buildAdminProspectDetailResponse(ctx, id)
}

// AdminUpdateSellerProspect 更新可編輯欄位。狀態與比對鍵的重算都在
// usecase.UpdateForAdmin 裡（它把 in.Status 釘死成 ACTIVE，Update 的 SQL
// 述詞只接受這個狀態），這裡只負責把 wire 欄位攤成 domain.SellerProspect。
//
// UpdateForAdmin 的 SQL 是 `:one` 查詢、述詞是 `status = 'ACTIVE'`：id 不
// 存在、或列存在但不是 ACTIVE（CONVERTED／ARCHIVED），兩種情況回的都是
// 同一個 pgx.ErrNoRows，無法從錯誤本身分辨是哪一種。不額外補一次查詢去
// 分辨——那會讓每一次更新都多付一趟往返，只為了讓錯誤訊息更精確一點。
// NotFound 對「id 不存在」是對的、對「非 ACTIVE」嚴格說不算精確，但訊息
// 用一句話涵蓋兩種讀法（同 ErrProspectMerged 的處理方式），不斷言是哪一
// 種——好過讓操作者看到「系統錯誤」。
//
// 這裡刻意不比照 buildAdminProspectDetailResponse／CreateInvite 把「非
// ErrNoRows」也包成 Internal：ValidateProspectFields 會回
// *usecase.ValidationError（一個 DomainCoder），必須原樣往上傳給既有
// ErrorInterceptor 映射成 InvalidArgument，這裡包了會連帶吃掉那個映射。
func (s *SellerServer) AdminUpdateSellerProspect(ctx context.Context, req *sellerpb.AdminUpdateSellerProspectRequest) (*sellerpb.AdminGetSellerProspectResponse, error) {
	id, err := parseProspectAdminID(req.GetId(), "id")
	if err != nil {
		return nil, err
	}

	in := domain.SellerProspect{
		ID:              id,
		DisplayName:     req.GetDisplayName(),
		VendorType:      req.GetVendorType(),
		OutreachStatus:  req.GetOutreachStatus(),
		PublicEmail:     req.GetPublicEmail(),
		PublicPhone:     req.GetPublicPhone(),
		PublicWebsite:   req.GetPublicWebsite(),
		PublicSocial:    req.GetPublicSocial(),
		Category:        req.GetCategory(),
		Region:          req.GetRegion(),
		ProductSummary:  req.GetProductSummary(),
		Positioning:     req.GetPositioning(),
		ResearchSummary: req.GetResearchSummary(),
		PriorityLetter:  req.GetPriorityLetter(),
		// 星等解不出來一律當 0，同 CreateSellerProspect 的既有處置：它只是
		// 排序的次要訊號，不值得讓一次更新因為這一欄格式不對而整筆失敗。
		PriorityStars: atoiOrZero(req.GetPriorityStars()),
	}
	if req.GetTaxId() != "" {
		taxID := req.GetTaxId()
		in.TaxID = &taxID
	}
	in.UpdatedByAccountID = parseOperatorAccountID(req.GetOperatorAccountId())

	updated, err := s.prospectAdmin.UpdateForAdmin(ctx, in)
	if err != nil {
		if nf := prospectNotFoundStatus(err, "查無此站外業者，或其狀態不允許編輯"); nf != nil {
			return nil, nf
		}
		if cs := prospectConflictStatus(err); cs != nil {
			return nil, cs
		}
		return nil, err
	}
	return &sellerpb.AdminGetSellerProspectResponse{Code: 0, Data: toAdminProspectItemPB(updated)}, nil
}

// AdminArchiveSellerProspect 封存或還原一筆站外業者，成功後回傳最新狀態
// （見 buildAdminProspectDetailResponse 的理由）。
//
// usecase.ErrProspectMerged 這個名字比它的語意窄：SetArchived 的
// rows == 0 可能來自「已被合併」「id 不存在」「封存一個 CONVERTED 列」
// 「還原一個本來就 ACTIVE 的列」四種情況中的任一種，SQL 述詞不細分是哪一
// 種。gRPC status 用 FailedPrecondition 沒有問題（四種都是「目前狀態不
// 允許」），但回應訊息不能斷言「已合併」——那在後三種情況下是對操作者
// 謊報事實，訊息必須涵蓋全部成因。
//
// 還原（archived = false）額外可能撞 uq_seller_prospects_tax_id——封存期間
// 別的業者合法拿走了同一個統編（見 repository.SetArchived 的註解）。這裡
// 用 prospectConflictStatus 收斂成 AlreadyExists，是該判斷的第五個呼叫端
// （另外四個是 CreateSellerProspect、AdminUpdateSellerProspect、
// AdminMergeSellerProspects、AdminConvertSellerProspect）。
func (s *SellerServer) AdminArchiveSellerProspect(ctx context.Context, req *sellerpb.AdminArchiveSellerProspectRequest) (*sellerpb.AdminGetSellerProspectResponse, error) {
	id, err := parseProspectAdminID(req.GetId(), "id")
	if err != nil {
		return nil, err
	}
	operatorID := parseOperatorAccountID(req.GetOperatorAccountId())

	if err := s.prospectAdmin.SetArchived(ctx, id, req.GetArchived(), operatorID); err != nil {
		if errors.Is(err, usecase.ErrProspectMerged) {
			return nil, status.Error(codes.FailedPrecondition, "這筆站外業者目前的狀態不允許這個操作")
		}
		if cs := prospectConflictStatus(err); cs != nil {
			return nil, cs
		}
		// 同 buildAdminProspectDetailResponse：沒被辨識的錯誤在這裡就直接
		// 包成 Internal，不要原樣往上傳給 ErrorInterceptor 決定——一個把
		// 所有錯誤都當 FailedPrecondition 的 handler 會讓上面的判斷看起來
		// 正確，但一次真正的 DB 故障會被呈現成「操作被拒」，操作者會以為
		// 是自己做錯了。
		return nil, status.Error(codes.Internal, err.Error())
	}
	return s.buildAdminProspectDetailResponse(ctx, id)
}

// AdminCreateProspectInvite 建立一筆邀請。有效天數解不出來或 <= 0 時，
// usecase.CreateInvite 自己套用服務端預設（30 天，見該函式註解），這裡
// 用既有 atoiOrZero 容錯——空字串與垃圾字串都落到同一個「讓 usecase 決定
// 預設值」的分支，不需要在這裡另外判斷。
//
// usecase.CreateInvite 第一步是 GetByID 確認 prospect 存在，查無資料時
// 一樣冒出 pgx.ErrNoRows。對一個不存在的 id 建邀請是呼叫端送錯了 id，是
// 客戶端錯誤不是伺服端故障，回 NotFound 而不是讓操作者看到「系統錯誤」
// 卻不知道問題出在哪。usecase.ErrProspectNotInvitable（已封存／被合併／
// 已入駐）是另一種客戶端可辨識的拒絕，回 FailedPrecondition：擋住這裡
// 才是合併第 3 步（搬邀請到 winner）真正要保護的不變式——不擋的話，一筆
// 已被合併掉但仍可經 ?status=ARCHIVED 開啟詳情頁的 loser，還是能繼續建
// 邀請、發連結，vendor 用它註冊時歸因悄悄失敗，token 卻已消費。
func (s *SellerServer) AdminCreateProspectInvite(ctx context.Context, req *sellerpb.AdminCreateProspectInviteRequest) (*sellerpb.AdminCreateProspectInviteResponse, error) {
	prospectID, err := parseProspectAdminID(req.GetProspectId(), "prospect_id")
	if err != nil {
		return nil, err
	}
	validDays := atoiOrZero(req.GetValidDays())

	invite, token, err := s.prospectAdmin.CreateInvite(ctx, prospectID, validDays)
	if err != nil {
		if nf := prospectNotFoundStatus(err, "查無此站外業者"); nf != nil {
			return nil, nf
		}
		if errors.Is(err, usecase.ErrProspectNotInvitable) {
			return nil, status.Error(codes.FailedPrecondition, "這筆站外業者目前的狀態不允許建立邀請")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &sellerpb.AdminCreateProspectInviteResponse{
		Code: 0, Data: toProspectInvitePB(invite), Token: token,
	}, nil
}

// AdminRevokeProspectInvite 撤銷一筆邀請。同時解析 prospect_id 與
// invite_id：usecase.RevokeInvite 的 SQL 述詞會比對兩者，任一對不上都回
// ErrProspectInviteNotRevocable——已消費的邀請不再可撤銷（那筆歸因已經
// 發生，撤銷它只會讓歷史說謊），這裡同樣不斷言是哪一種成因，只回「目前
// 狀態不允許」。
func (s *SellerServer) AdminRevokeProspectInvite(ctx context.Context, req *sellerpb.AdminRevokeProspectInviteRequest) (*sellerpb.AdminRevokeProspectInviteResponse, error) {
	prospectID, err := parseProspectAdminID(req.GetProspectId(), "prospect_id")
	if err != nil {
		return nil, err
	}
	inviteID, err := parseProspectAdminID(req.GetInviteId(), "invite_id")
	if err != nil {
		return nil, err
	}

	if err := s.prospectAdmin.RevokeInvite(ctx, prospectID, inviteID); err != nil {
		if errors.Is(err, usecase.ErrProspectInviteNotRevocable) {
			return nil, status.Error(codes.FailedPrecondition, "這筆邀請目前的狀態不允許撤銷")
		}
		// 同 AdminArchiveSellerProspect／AdminGetSellerProspect：沒被辨識的
		// 錯誤在這裡直接包成 Internal。
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &sellerpb.AdminRevokeProspectInviteResponse{Code: 0}, nil
}

// AdminListDuplicateProspects 掃出全表的 T2 疑似重複配對（usecase.ListDuplicates
// 的理由見該函式註解：T1／T3 不在這份清單裡）。truncated 必須誠實回報——
// admin 看到 500 對而不知道還有更多，會以為自己已經處理完了，而「重複清單
// 回空」正是本輪 dev 驗收的判準，謊報會讓驗收在錯的時間點通過。
func (s *SellerServer) AdminListDuplicateProspects(ctx context.Context, _ *sellerpb.AdminListDuplicateProspectsRequest) (*sellerpb.AdminListDuplicateProspectsResponse, error) {
	pairs, truncated, err := s.prospectAdmin.ListDuplicates(ctx)
	if err != nil {
		return nil, err
	}

	data := make([]*sellerpb.AdminProspectDuplicatePair, 0, len(pairs))
	for _, p := range pairs {
		data = append(data, &sellerpb.AdminProspectDuplicatePair{
			Left:      toAdminProspectItemPB(p.Left),
			Right:     toAdminProspectItemPB(p.Right),
			MatchedOn: p.MatchedOn,
		})
	}
	return &sellerpb.AdminListDuplicateProspectsResponse{
		Code: 0, Data: data, Truncated: truncated,
	}, nil
}

// AdminMergeSellerProspects 把 loser 併進 winner，回傳合併後的 winner
// 最新狀態。四種拒絕情境（loser 已入駐／已封存／列不存在／winner 等於
// loser，見 domain.SellerProspectRepository.Merge 的文件註解）都由
// repository 包成 usecase.ErrProspectMergeNotAllowed，這裡轉成
// FailedPrecondition：合併被拒不是系統故障，是「這家業者已經入駐了，不能
// 被合併掉」，那個區別正是這個畫面需要人來操作的理由。沒有包 sentinel 的
// 錯誤維持 Internal——一個真正的 DB 故障不能被呈現成「操作被拒」，讓操作者
// 以為是自己做錯了。
//
// err.Error() 帶四種情境各自的具體原因（repository.Merge 用
// fmt.Errorf("%w: <原因>", ErrProspectMergeNotAllowed, ...) 組出來，見該
// 函式文件註解）：這裡原樣附在訊息後面，不是隨手丟掉——repository 特地為
// 四種情境各寫一句話，就是要操作者知道自己撞的是哪一種，全部壓成同一句
// 「不被允許」會讓那份區分白費。
func (s *SellerServer) AdminMergeSellerProspects(ctx context.Context, req *sellerpb.AdminMergeSellerProspectsRequest) (*sellerpb.AdminMergeSellerProspectsResponse, error) {
	winnerID, err := parseProspectAdminID(req.GetWinnerId(), "winner_id")
	if err != nil {
		return nil, err
	}
	loserID, err := parseProspectAdminID(req.GetLoserId(), "loser_id")
	if err != nil {
		return nil, err
	}
	operatorID := parseOperatorAccountID(req.GetOperatorAccountId())

	winner, err := s.prospectAdmin.Merge(ctx, winnerID, loserID, operatorID)
	if err != nil {
		if errors.Is(err, usecase.ErrProspectMergeNotAllowed) {
			return nil, status.Error(codes.FailedPrecondition, "這個合併操作目前不被允許："+err.Error())
		}
		if cs := prospectConflictStatus(err); cs != nil {
			return nil, cs
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &sellerpb.AdminMergeSellerProspectsResponse{Code: 0, Data: toAdminProspectItemPB(winner)}, nil
}

// AdminConvertSellerProspect 是歸因 spec §5.2 第二、三條手動綁定路徑，
// 成功後回傳最新狀態（見 buildAdminProspectDetailResponse 的理由）。兩種
// 拒絕情境落在不同 status 上，不能都回同一個碼：usecase.ErrStoreNotFound
// （store 不存在或非 current stint）是呼叫端送錯了 store_id，是客戶端
// 錯誤，回 InvalidArgument；usecase.ErrProspectNotConvertible（已入駐、
// 已封存或已被合併）是這筆列目前的狀態不允許再次歸因，回
// FailedPrecondition。兩者都不是 Internal——回 Internal 會讓 admin 看到
// 「系統錯誤」而不是「這筆已經入駐了」，那正是操作者需要知道的事實。
//
// 這是四支 admin 寫入 RPC（Update／Archive／Merge／Convert）裡最後一支才
// 讀 operator_account_id 的——人工歸因決定一筆真實 store 認哪一筆 prospect
// 當前身，不可逆，spec §5.3／§6 都要求記下是誰做的。同 Update／Archive／
// Merge 的既有處置，格式錯誤或 0 一律視為「沒有」（parseOperatorAccountID），
// 不讓這一欄的格式問題擋下整個歸因操作。
func (s *SellerServer) AdminConvertSellerProspect(ctx context.Context, req *sellerpb.AdminConvertSellerProspectRequest) (*sellerpb.AdminGetSellerProspectResponse, error) {
	id, err := parseProspectAdminID(req.GetId(), "id")
	if err != nil {
		return nil, err
	}
	storeID, err := parseProspectAdminID(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	operatorID := parseOperatorAccountID(req.GetOperatorAccountId())

	if err := s.prospectAdmin.ConvertForAdmin(ctx, id, storeID, operatorID); err != nil {
		switch {
		case errors.Is(err, usecase.ErrStoreNotFound):
			return nil, status.Error(codes.InvalidArgument, "指定的 store 不存在或非現任租期")
		case errors.Is(err, usecase.ErrProspectNotConvertible):
			return nil, status.Error(codes.FailedPrecondition, "這筆站外業者目前的狀態不允許歸因")
		default:
			if cs := prospectConflictStatus(err); cs != nil {
				return nil, cs
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return s.buildAdminProspectDetailResponse(ctx, id)
}

// AdminListProspectTaxIdMatches 是歸因 spec §5.2 第三條路徑的清單頁：統編
// 對得上、但從未被連結的 prospect／已入駐賣家配對（domain.ProspectTaxIDMatch
// 的理由見該型別註解）。
func (s *SellerServer) AdminListProspectTaxIdMatches(ctx context.Context, _ *sellerpb.AdminListProspectTaxIdMatchesRequest) (*sellerpb.AdminListProspectTaxIdMatchesResponse, error) {
	matches, err := s.prospectAdmin.ListTaxIDMatches(ctx)
	if err != nil {
		return nil, err
	}

	data := make([]*sellerpb.AdminProspectTaxIdMatch, 0, len(matches))
	for _, m := range matches {
		data = append(data, &sellerpb.AdminProspectTaxIdMatch{
			Prospect:                 toAdminProspectItemPB(m.Prospect),
			StoreId:                  formatID(m.StoreID),
			StoreName:                m.StoreName,
			CertificationCompanyName: m.CertificationCompanyName,
			CertificationStatus:      m.CertificationStatus,
		})
	}
	return &sellerpb.AdminListProspectTaxIdMatchesResponse{Code: 0, Data: data}, nil
}

// AdminUpdateSellerProspectMedia 換一張品牌圖片，成功後回傳最新狀態——與
// AdminArchiveSellerProspect／AdminConvertSellerProspect 同一個回應型別與同一條
// 理由（見 buildAdminProspectDetailResponse）。Admin 收到後會逐項核對 key、
// version 與 source_url 才認定存檔成功，少回欄位等同存檔失敗。
//
// 錯誤分三類，不能合併成一種：
//   - kind／來源網址不合法 ⇒ InvalidArgument，呼叫端改輸入就能過。
//   - ErrProspectMediaConflict ⇒ FailedPrecondition。版本不符、列不是 ACTIVE、
//     已被合併三者共用這一個出口，訊息只說「請重新讀取」不斷言是哪一種——理由
//     同 AdminArchiveSellerProspect 那段：SQL 述詞不細分，斷言就會謊報。
//   - 其餘 ⇒ Internal。media-service 不可達或 link 失敗屬於這一類，不能呈現成
//     「操作被拒」讓操作者以為是自己做錯了。
func (s *SellerServer) AdminUpdateSellerProspectMedia(ctx context.Context, req *sellerpb.AdminUpdateSellerProspectMediaRequest) (*sellerpb.AdminGetSellerProspectResponse, error) {
	id, err := parseProspectAdminID(req.GetId(), "id")
	if err != nil {
		return nil, err
	}
	// expected_media_version 缺值不視為「不在意版本」——那會讓兩個 admin 的換圖
	// 互相覆蓋。解析失敗一律當客戶端錯誤，不退回 0。
	expected, err := parseProspectAdminID(req.GetExpectedMediaVersion(), "expected_media_version")
	if err != nil {
		return nil, err
	}

	if _, err := s.prospectAdmin.UpdateProspectMedia(ctx, usecase.ProspectMediaInput{
		ProspectID:           id,
		Kind:                 req.GetKind(),
		StagedToken:          req.GetStagedToken(),
		SourceURL:            req.GetSourceUrl(),
		ExpectedMediaVersion: expected,
		OperatorAccountID:    parseOperatorAccountID(req.GetOperatorAccountId()),
	}); err != nil {
		switch {
		case errors.Is(err, usecase.ErrProspectMediaKindInvalid),
			errors.Is(err, usecase.ErrProspectMediaSourceURLInvalid):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, domain.ErrProspectMediaConflict):
			return nil, status.Error(codes.FailedPrecondition, "這筆站外業者的品牌圖片已被其他人更新或目前狀態不允許編輯，請重新讀取後再試")
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// 換圖不動邀請歷史，但回應型別帶 invites，所以照既有做法重查一次詳情
	// （同 AdminArchiveSellerProspect），讓 Admin 拿到完整且一致的一筆，
	// 而不是缺一半的投影。
	return s.buildAdminProspectDetailResponse(ctx, id)
}
