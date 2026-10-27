package handler

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	"github.com/Will413028/hawkr/gateway/internal/response"
	pricebookpb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"github.com/gin-gonic/gin"
)

// SearchCropCatalog searches the full crop/crop_group catalog for a picker
// UI's autocomplete.
// @Summary      搜尋品項目錄
// @Description  空字串回傳常見品項（種子表 27 筆）；輸入後比對官方名稱與俗名別名
// @Tags         賣家市場行情 (Seller Pricebook)
// @Produce      json
// @Security     BearerAuth
// @Param        q      query  string  false  "搜尋字串，留空回傳常見品項"
// @Param        limit  query  int     false  "回傳筆數上限，預設 20"
// @Success      200  {object}  object{code=int,data=object}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/sellers/pricebook/crop-catalog/search [get]
func (h *Holder) SearchCropCatalog(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Unauthorized(c)
		return
	}
	query := strings.TrimSpace(c.Query("q"))
	limit, _ := strconv.Atoi(c.Query("limit"))

	resp, err := h.Pricebook.SearchCropCatalog(c.Request.Context(), query, int32(limit))
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	response.OKProto(c, resp)
}

// GetObservedPriceHistory returns raw historical observed prices for a
// resolved crop or crop group.
// @Summary      取得歷史實際成交行情
// @Description  查詢某品項最近一段時間（伺服器預設窗口）的實際成交價；cropName/cropCode/groupCode 三選一必填
// @Tags         賣家市場行情 (Seller Pricebook)
// @Produce      json
// @Security     BearerAuth
// @Param        cropName    query  string  false  "賣家端品項名稱；與 cropCode/groupCode 三選一必填"
// @Param        cropCode    query  string  false  "品項代碼，直接指定子品項；與 cropName/groupCode 三選一必填"
// @Param        groupCode   query  string  false  "大類代碼，直接指定大類；與 cropName/cropCode 三選一必填"
// @Param        marketCode  query  string  false  "市場代碼，選填"
// @Param        history     query  string  false  "true=回傳完整區間；預設 false 只回最新一筆"
// @Success      200  {object}  object{code=int,data=object}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/sellers/pricebook/observed-price [get]
func (h *Holder) GetObservedPriceHistory(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Unauthorized(c)
		return
	}
	cropName := strings.TrimSpace(c.Query("cropName"))
	cropCode := strings.TrimSpace(c.Query("cropCode"))
	groupCode := strings.TrimSpace(c.Query("groupCode"))
	if cropName == "" && cropCode == "" && groupCode == "" {
		response.BadRequest(c, "one of cropName, cropCode, or groupCode is required")
		return
	}
	marketCode := strings.TrimSpace(c.Query("marketCode"))
	history, _ := strconv.ParseBool(c.Query("history"))

	resp, err := h.Pricebook.GetObservedPrice(c.Request.Context(), cropName, marketCode, cropCode, groupCode, history)
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	response.OKProto(c, resp)
}

// GetForecastCurve returns the full 7-day forecast curve for a resolved
// crop or crop group.
// @Summary      取得 7 日預測曲線
// @Description  查詢某品項未來 7 天的 P10/P50/P90 預測（TWD/kg，未換算單件價）；cropName/cropCode/groupCode 三選一必填
// @Tags         賣家市場行情 (Seller Pricebook)
// @Produce      json
// @Security     BearerAuth
// @Param        cropName    query  string  false  "賣家端品項名稱；與 cropCode/groupCode 三選一必填"
// @Param        cropCode    query  string  false  "品項代碼，直接指定子品項；與 cropName/groupCode 三選一必填"
// @Param        groupCode   query  string  false  "大類代碼，直接指定大類；與 cropName/cropCode 三選一必填"
// @Param        marketCode  query  string  false  "市場代碼；特定子品項省略時回全市場成交量加權預測，大類選填僅排除比較市場"
// @Success      200  {object}  object{code=int,data=object}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/sellers/pricebook/forecast-curve [get]
func (h *Holder) GetForecastCurve(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Unauthorized(c)
		return
	}
	cropName := strings.TrimSpace(c.Query("cropName"))
	cropCode := strings.TrimSpace(c.Query("cropCode"))
	groupCode := strings.TrimSpace(c.Query("groupCode"))
	if cropName == "" && cropCode == "" && groupCode == "" {
		response.BadRequest(c, "one of cropName, cropCode, or groupCode is required")
		return
	}
	marketCode := strings.TrimSpace(c.Query("marketCode"))

	resp, err := h.Pricebook.GetForecastCurve(c.Request.Context(), cropName, marketCode, cropCode, groupCode)
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	data, err := forecastCurveHTTPData(resp)
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	response.OK(c, data)
}

// Preserve ProtoJSON field presence everywhere except comparison.p50, whose
// HTTP contract explicitly represents an unavailable supplementary price as null.
func forecastCurveHTTPData(resp *pricebookpb.GetForecastCurveResponse) (map[string]json.RawMessage, error) {
	var data map[string]json.RawMessage
	if err := json.Unmarshal(response.ProtoToRaw(resp), &data); err != nil {
		return nil, err
	}
	if len(resp.GetMarketComparison()) == 0 {
		return data, nil
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(data["marketComparison"], &entries); err != nil {
		return nil, err
	}
	for i, entry := range resp.GetMarketComparison() {
		if entry.P50 == nil {
			entries[i]["p50"] = json.RawMessage("null")
		}
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return nil, err
	}
	data["marketComparison"] = raw
	return data, nil
}

// GetTrendSummary returns a day/week trend series for a resolved crop or
// crop group, with an optional WoW/364-day-aligned-YoY comparison.
// @Summary      取得行情趨勢摘要
// @Description  查詢某品項在區間內的日/週趨勢，可選 WoW/364天對齊 YoY 比較；cropCode/groupCode 二選一必填
// @Tags         賣家市場行情 (Seller Pricebook)
// @Produce      json
// @Security     BearerAuth
// @Param        cropCode     query  string  false  "品項代碼；與 groupCode 二選一必填，需搭配 marketCode"
// @Param        groupCode    query  string  false  "大類代碼；與 cropCode 二選一必填，不可搭配 marketCode"
// @Param        marketCode   query  string  false  "市場代碼——cropCode 必填，groupCode 禁填"
// @Param        from         query  string  true   "起始日期 YYYY-MM-DD"
// @Param        to           query  string  true   "結束日期 YYYY-MM-DD，區間不可超過 366 天"
// @Param        granularity  query  string  true   "day 或 week"
// @Param        comparisons  query  string  false  "逗號分隔，子集為 wow,yoy364"
// @Success      200  {object}  object{code=int,data=object}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/sellers/pricebook/trend-summary [get]
func (h *Holder) GetTrendSummary(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Unauthorized(c)
		return
	}
	cropCode := strings.TrimSpace(c.Query("cropCode"))
	groupCode := strings.TrimSpace(c.Query("groupCode"))
	if cropCode == "" && groupCode == "" {
		response.BadRequest(c, "one of cropCode or groupCode is required")
		return
	}
	if cropCode != "" && groupCode != "" {
		response.BadRequest(c, "exactly one of cropCode or groupCode must be set")
		return
	}
	marketCode := strings.TrimSpace(c.Query("marketCode"))
	from := strings.TrimSpace(c.Query("from"))
	to := strings.TrimSpace(c.Query("to"))
	granularity := strings.TrimSpace(c.Query("granularity"))
	var comparisons []string
	if raw := strings.TrimSpace(c.Query("comparisons")); raw != "" {
		comparisons = strings.Split(raw, ",")
	}

	resp, err := h.Pricebook.GetTrendSummary(c.Request.Context(), cropCode, groupCode, marketCode, from, to, granularity, comparisons)
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	response.OKProto(c, resp)
}

// spatialSurfaceCacheMaxAgeSeconds is a conservative starting point pending
// the live performance measurement (design doc §Caching/Compression):
// short enough that a target_date day-rollover or a redeploy is absorbed
// within a minute, long enough to dedupe a seller flipping horizonDays back
// and forth in the same map session.
const spatialSurfaceCacheMaxAgeSeconds = 60

// GetSpatialSurface returns the nationwide price surface for a crop or crop
// group at a single horizon (issue #2248): market points, an interpolated
// price surface, its contours, and the per-county rollup, as RFC 7946
// GeoJSON. The response is deterministic for the same
// (selection, horizonDays, target date, boundary/model version), so it
// carries a content-hash ETag and a short private Cache-Control, and is
// gzipped — the GeoJSON payload runs into the hundreds of KB.
// @Summary      取得空間行情曲面
// @Description  查詢某品項未來 N 天的全國價格空間分布（市場點位、內插曲面、等高線、縣市 rollup）；cropCode/groupCode 二選一必填
// @Tags         賣家市場行情 (Seller Pricebook)
// @Produce      json
// @Security     BearerAuth
// @Param        cropCode     query  string  false  "品項代碼；與 groupCode 二選一必填"
// @Param        groupCode    query  string  false  "大類代碼；與 cropCode 二選一必填"
// @Param        horizonDays  query  int     true   "預測天數，1-7"
// @Success      200  {object}  object{code=int,data=object}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/sellers/pricebook/spatial-surface [get]
func (h *Holder) GetSpatialSurface(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Unauthorized(c)
		return
	}
	cropCode := strings.TrimSpace(c.Query("cropCode"))
	groupCode := strings.TrimSpace(c.Query("groupCode"))
	if cropCode == "" && groupCode == "" {
		response.BadRequest(c, "one of cropCode or groupCode is required")
		return
	}
	if cropCode != "" && groupCode != "" {
		response.BadRequest(c, "exactly one of cropCode or groupCode must be set")
		return
	}
	horizonDays, _ := strconv.Atoi(c.Query("horizonDays"))

	resp, err := h.Pricebook.GetSpatialSurface(c.Request.Context(), cropCode, groupCode, int32(horizonDays))
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}

	payload := response.ProtoToRaw(resp)
	etag := weakETag(payload)
	c.Header("ETag", etag)
	c.Header("Cache-Control", fmt.Sprintf("private, max-age=%d", spatialSurfaceCacheMaxAgeSeconds))
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	writeGzippedIfAccepted(c, payload)
}

// weakETag hashes the already-serialized payload bytes rather than
// composing a key from lineage/scale/boundary digests: hashing the ~1MB
// body costs single-digit milliseconds (negligible next to this handler's
// measured ~300ms compute), and a composite key has the opposite failure
// mode — missing one field that can legitimately vary silently turns into a
// stale 304 (design doc §Caching/Compression trade-off).
func weakETag(payload []byte) string {
	sum := sha256.Sum256(payload)
	return `W/"` + hex.EncodeToString(sum[:]) + `"`
}

// writeGzippedIfAccepted gzips the envelope only when the client advertises
// support — scoped to this one handler (not a route-group or gateway-wide
// middleware): this gateway also serves SSE/WebSocket routes that must
// never be wrapped in a compressing ResponseWriter, and no other pricebook
// route has a payload large enough to need this. Buffers in memory rather
// than streaming through a wrapped ResponseWriter — the payload is bounded
// (low hundreds of KB to ~1MB), so the simpler, harder-to-get-wrong
// buffered approach costs nothing measurable and avoids Content-Length/
// flush-ordering pitfalls a streaming gzip.Writer wrapped around
// gin.ResponseWriter would introduce.
//
// Compresses at gzip.BestSpeed, not the zlib default: on a 1MB+ payload,
// compression time on a shared gateway process is not negligible, and
// measurement showed BestSpeed trades a ~8.5% larger body for roughly 5x
// less CPU time versus DefaultCompression at this payload size — the right
// trade for a request-path handler serving every caller, not a batch job
// that can spend CPU freely to shave a few percent off the wire size.
func writeGzippedIfAccepted(c *gin.Context, payload json.RawMessage) {
	if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
		response.OK(c, payload)
		return
	}
	// 🔴 這條路徑自己組 Envelope 再用 c.Data 寫出，繞過 internal/response 的所有出口
	// ⇒ bizcarrier 看不到它（carrier.go 檔頭失效模式 4）。兩條 ruleguard lint 都不命中：
	// banDirectJSON 認的是 c.JSON、banEnvelopeSuccessEmitters 認的是 envelope.OK/Page/…。
	// 所以這裡必須自己問一次，否則本請求若有未消化的業務碼，會被寫成 code:0 的假成功。
	code, msg, rewrite := bizcarrier.Emit(c, 0)
	if !rewrite {
		code, msg = 0, "success"
	}
	body, _ := json.Marshal(response.Envelope{Code: code, Message: msg, Data: payload})
	var buf bytes.Buffer
	// gzip.NewWriterLevel only errors on an invalid level constant;
	// gzip.BestSpeed is always valid, so discarding the error here is safe
	// and matches this function's existing error-handling style, which
	// already discards gz.Write/gz.Close errors on an in-memory buffer for
	// the same reason.
	gz, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	_, _ = gz.Write(body)
	_ = gz.Close()
	c.Header("Content-Encoding", "gzip")
	c.Header("Vary", "Accept-Encoding")
	c.Data(http.StatusOK, "application/json; charset=utf-8", buf.Bytes())
}

// GetRegionalComparison returns a per-county price comparison table for a
// crop or crop group at a single horizon (issue #2249 Slice C). Unlike
// GetSpatialSurface, the payload is a small per-county table (no
// GeoJSON), so this skips ETag/gzip — same simple pattern as
// GetTrendSummary.
// @Summary      取得縣市行情比較表
// @Description  查詢某品項未來 N 天的縣市級價格比較（p10/p50/p90）；cropCode/groupCode 二選一必填
// @Tags         賣家市場行情 (Seller Pricebook)
// @Produce      json
// @Security     BearerAuth
// @Param        cropCode     query  string  false  "品項代碼；與 groupCode 二選一必填"
// @Param        groupCode    query  string  false  "大類代碼；與 cropCode 二選一必填"
// @Param        horizonDays  query  int     true   "預測天數，1-7"
// @Success      200  {object}  object{code=int,data=object}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/sellers/pricebook/regional-comparison [get]
func (h *Holder) GetRegionalComparison(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Unauthorized(c)
		return
	}
	cropCode := strings.TrimSpace(c.Query("cropCode"))
	groupCode := strings.TrimSpace(c.Query("groupCode"))
	if cropCode == "" && groupCode == "" {
		response.BadRequest(c, "one of cropCode or groupCode is required")
		return
	}
	if cropCode != "" && groupCode != "" {
		response.BadRequest(c, "exactly one of cropCode or groupCode must be set")
		return
	}
	horizonDays, _ := strconv.Atoi(c.Query("horizonDays"))

	resp, err := h.Pricebook.GetRegionalComparison(c.Request.Context(), cropCode, groupCode, int32(horizonDays))
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	response.OKProto(c, resp)
}

// ResolveCategoryMapping resolves a product-service L3 categoryId to its
// @Summary      解析分類對應的 Pricebook 品項
// @Tags         賣家市場行情 (Seller Pricebook)
// @Produce      json
// @Security     BearerAuth
// @Param        category_id  query  string  true  "L3 分類 ID"
// @Success      200  {object}  object{code=int,data=object}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/sellers/pricebook/category-resolution [get]
func (h *Holder) ResolveCategoryMapping(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Unauthorized(c)
		return
	}
	categoryID := strings.TrimSpace(c.Query("category_id"))
	if categoryID == "" {
		response.BadRequest(c, "category_id is required")
		return
	}
	resp, err := h.Pricebook.ResolveCategoryMapping(c.Request.Context(), categoryID)
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	response.OKProto(c, resp)
}

// GetCategoryPriceRollup returns the daily-computed price snapshot for a
// product-service category at any tree level (L1/L2/L3).
// @Summary      取得分類彙總價格
// @Description  category_id 可以是分類樹任一層；status=unavailable 代表底下沒有任何有價分類，不是查詢失敗
// @Tags         賣家市場行情 (Seller Pricebook)
// @Produce      json
// @Security     BearerAuth
// @Param        category_id  query  string  true  "分類 ID（任一層）"
// @Success      200  {object}  object{code=int,data=object}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/sellers/pricebook/category-price-rollup [get]
func (h *Holder) GetCategoryPriceRollup(c *gin.Context) {
	if _, ok := getAccountID(c); !ok {
		response.Unauthorized(c)
		return
	}
	categoryID := strings.TrimSpace(c.Query("category_id"))
	if categoryID == "" {
		response.BadRequest(c, "category_id is required")
		return
	}
	resp, err := h.Pricebook.GetCategoryPriceRollup(c.Request.Context(), categoryID)
	if err != nil {
		response.GrpcErr(c, "pricebook", err)
		return
	}
	response.OKProto(c, resp)
}
