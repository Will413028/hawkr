// Package bizcalendar 是 BizCalendarAdminService 的 handler：工作日行事曆（platform/bizcalendar）的匯入與涵蓋狀態。
package bizcalendar

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/date"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/bizcalendar/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/bizcalendar/v1alpha1/bizcalendarv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/bizcalendar"
)

// Source 是 handler 需要的行事曆方法（*bizcalendar.Store）。
type Source interface {
	Import(ctx context.Context, actor adminaudit.Actor, year int, content string) (bizcalendar.ImportResult, error)
	Load(ctx context.Context) (*bizcalendar.Calendar, error)
}

// Pusher 把目前的行事曆推給 invoice-service（*bizcalendar.Mirror）。
type Pusher interface {
	Push(ctx context.Context) error
}

// Admin 實作 BizCalendarAdminService。授權（只有管理員、各方法的權限）由 rpc 的 interceptor 與 adminauth 的宣告表檢查。
type Admin struct {
	src    Source
	pusher Pusher
	log    *slog.Logger
}

var _ bizcalendarv1alpha1connect.BizCalendarAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler。
func NewAdmin(src Source) *Admin { return &Admin{src: src, log: slog.New(slog.DiscardHandler)} }

// WithMirror 讓每次匯入成功後把行事曆推給 invoice-service。推送放在這一層而不在 platform 的 Import 裡：Import 的 transaction 只管資料與後台紀錄，
// 對舊服務的呼叫是遷移期的組裝，隨 A5 一起刪掉時不碰 platform。
func (h *Admin) WithMirror(p Pusher, log *slog.Logger) *Admin {
	h.pusher, h.log = p, log
	return h
}

// ImportHolidayCalendar 匯入一年的辦公日曆表；操作的管理員從 ctx 取、動作是 procedure 名稱，後台紀錄與資料在 platform 的同一個 transaction。
func (h *Admin) ImportHolidayCalendar(ctx context.Context, req *connect.Request[adminpb.ImportHolidayCalendarRequest]) (*connect.Response[adminpb.ImportHolidayCalendarResponse], error) {
	admin, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	res, err := h.src.Import(ctx, adminaudit.Actor{AdminID: admin, Action: req.Spec().Procedure}, int(req.Msg.GetYear()), req.Msg.GetCsvContent())
	if err != nil {
		return nil, err
	}
	h.push(ctx)
	return connect.NewResponse(&adminpb.ImportHolidayCalendarResponse{
		Year:             &adminpb.HolidayCalendarYear{Year: req.Msg.GetYear(), DayCount: int32(res.DayCount), HolidayCount: int32(res.HolidayCount)}, //nolint:gosec // 一年最多 366 天
		ReplacedDayCount: int32(res.ReplacedDayCount),                                                                                                 //nolint:gosec // 同上
	}), nil
}

// GetHolidayCalendarCoverage 讀已匯入的年份。
func (h *Admin) GetHolidayCalendarCoverage(ctx context.Context, _ *connect.Request[adminpb.GetHolidayCalendarCoverageRequest]) (*connect.Response[adminpb.GetHolidayCalendarCoverageResponse], error) {
	cal, err := h.src.Load(ctx)
	if err != nil {
		return nil, err
	}
	out := &adminpb.GetHolidayCalendarCoverageResponse{}
	for _, y := range cal.Coverage() {
		out.Years = append(out.Years, &adminpb.HolidayCalendarYear{Year: int32(y.Year), DayCount: int32(y.DayCount), HolidayCount: int32(y.HolidayCount)}) //nolint:gosec // 年份在 1..9999、天數至多 366
	}
	if last, ok := cal.LastCoveredDate(); ok {
		out.LastCoveredDate = &date.Date{Year: int32(last.Year()), Month: int32(last.Month()), Day: int32(last.Day())} //nolint:gosec // time.Time 的日曆欄位在 Date 的範圍內
	}
	return connect.NewResponse(out), nil
}

// push 在匯入已提交之後推送。資料的擁有者是這裡，推送失敗不讓匯入失敗：只記 log，下一次每日對帳重送。
// 用 WithoutCancel：操作者關掉頁面不該中斷已經開始的推送。
func (h *Admin) push(ctx context.Context) {
	if h.pusher == nil {
		return
	}
	if err := h.pusher.Push(context.WithoutCancel(ctx)); err != nil {
		h.log.Error("匯入後推送行事曆給 invoice-service 失敗，等每日對帳重送", "error", err)
	}
}
