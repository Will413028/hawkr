// Package description 驗證與編碼商品描述的結構化區塊：段落（純文字）、圖片（引用 product_images 的資產）、項目清單、表格。
// 這裡是純函式：不碰資料庫與 media；圖片的 Confirm 與引用表在 product。
package description

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// Normalize 驗證區塊並回傳整理過的副本（文字去頭尾空白）。欄位路徑以 field 為前綴（例如 "Description"）。
// 圖片區塊必須剛好給 Token 或 AssetID 其中一個，alt 不超過上限；上限見 model 的常數。
// 文字沒有單段上限，段落、清單項目與表格儲存格的文字合計不得超過 model.MaxDescriptionRunes（不含 alt）；清單項目與儲存格另有各自的上限。
func Normalize(field string, blocks []model.Block) ([]model.Block, error) {
	if len(blocks) > model.MaxDescriptionBlocks {
		return nil, model.Invalid(field, model.ReasonTooMany)
	}
	out := make([]model.Block, 0, len(blocks))
	images := 0
	budget := textBudget{field: field}
	for i, b := range blocks {
		f := fmt.Sprintf("%s[%d]", field, i)
		switch b.Kind {
		case model.BlockParagraph:
			text := trimtext.Trim(b.Text)
			if err := checkText(f+".Text", text, 0, false); err != nil {
				return nil, err
			}
			if err := budget.add(text); err != nil {
				return nil, err
			}
			out = append(out, model.Block{Kind: b.Kind, Text: text})
		case model.BlockImage:
			if b.Image == nil {
				return nil, model.Invalid(f+".Image", model.ReasonRequired)
			}
			images++
			if images > model.MaxDescriptionImages {
				return nil, model.Invalid(field, model.ReasonTooMany)
			}
			img, err := NormalizeImage(f+".Image", *b.Image)
			if err != nil {
				return nil, err
			}
			out = append(out, model.Block{Kind: b.Kind, Image: &img})
		case model.BlockList:
			if len(b.Items) == 0 {
				return nil, model.Invalid(f+".Items", model.ReasonRequired)
			}
			if len(b.Items) > model.MaxListItems {
				return nil, model.Invalid(f+".Items", model.ReasonTooMany)
			}
			items := make([]string, len(b.Items))
			for j, it := range b.Items {
				items[j] = trimtext.Trim(it)
				if err := checkText(fmt.Sprintf("%s.Items[%d]", f, j), items[j], model.MaxListItemRunes, false); err != nil {
					return nil, err
				}
				if err := budget.add(items[j]); err != nil {
					return nil, err
				}
			}
			out = append(out, model.Block{Kind: b.Kind, Items: items})
		case model.BlockTable:
			rows, err := normalizeTable(f+".Rows", b.Rows, &budget)
			if err != nil {
				return nil, err
			}
			out = append(out, model.Block{Kind: b.Kind, Rows: rows})
		default:
			return nil, model.Invalid(f+".Kind", model.ReasonUnknown)
		}
	}
	return out, nil
}

// NormalizeImage 驗證一張圖的引用：Token 與 AssetID 剛好一個有值，alt 不超過上限。
func NormalizeImage(field string, img model.Image) (model.Image, error) {
	hasToken, hasAsset := img.Token != "", img.AssetID != uuid.Nil()
	if hasToken == hasAsset {
		return model.Image{}, model.Invalid(field, model.ReasonInvalid)
	}
	alt := trimtext.Trim(img.Alt)
	if utf8.RuneCountInString(alt) > model.MaxAltRunes {
		return model.Image{}, model.Invalid(field+".Alt", model.ReasonTooLong)
	}
	return model.Image{Token: img.Token, AssetID: img.AssetID, Alt: alt}, nil
}

func normalizeTable(field string, rows [][]string, budget *textBudget) ([][]string, error) {
	if len(rows) == 0 {
		return nil, model.Invalid(field, model.ReasonRequired)
	}
	if len(rows) > model.MaxTableRows {
		return nil, model.Invalid(field, model.ReasonTooMany)
	}
	width := len(rows[0])
	if width == 0 {
		return nil, model.Invalid(field, model.ReasonRequired)
	}
	if width > model.MaxTableCols {
		return nil, model.Invalid(field, model.ReasonTooMany)
	}
	out := make([][]string, len(rows))
	for r, row := range rows {
		if len(row) != width {
			return nil, model.Invalid(fmt.Sprintf("%s[%d]", field, r), model.ReasonInvalid)
		}
		out[r] = make([]string, width)
		for c, cell := range row {
			out[r][c] = trimtext.Trim(cell)
			if err := checkText(fmt.Sprintf("%s[%d][%d]", field, r, c), out[r][c], model.MaxTableCellRunes, true); err != nil {
				return nil, err
			}
			if err := budget.add(out[r][c]); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// textBudget 累計整份描述的文字量（rune），超過 model.MaxDescriptionRunes 回 TooLong，欄位是整份描述的欄位路徑。
type textBudget struct {
	field string
	used  int
}

func (b *textBudget) add(s string) error {
	b.used += utf8.RuneCountInString(s)
	if b.used > model.MaxDescriptionRunes {
		return model.Invalid(b.field, model.ReasonTooLong)
	}
	return nil
}

// checkText 檢查單一文字：max 大於 0 時是這段文字自己的上限，0 表示沒有單段上限。
func checkText(field, s string, max int, allowEmpty bool) error {
	if s == "" && !allowEmpty {
		return model.Invalid(field, model.ReasonRequired)
	}
	if max > 0 && utf8.RuneCountInString(s) > max {
		return model.Invalid(field, model.ReasonTooLong)
	}
	return nil
}

// ImageRefs 依出現順序回傳區塊裡的圖片引用（指標指向 blocks 內的元素，供呼叫方把 Confirm 的結果寫回）。
func ImageRefs(blocks []model.Block) []*model.Image {
	var out []*model.Image
	for i := range blocks {
		if blocks[i].Kind == model.BlockImage && blocks[i].Image != nil {
			out = append(out, blocks[i].Image)
		}
	}
	return out
}

// Text 回傳描述的純文字（段落、清單項目與表格儲存格，以空白相接），給 searchable_text。圖片不貢獻文字。
func Text(blocks []model.Block) string {
	var parts []string
	for _, b := range blocks {
		switch b.Kind {
		case model.BlockParagraph:
			parts = append(parts, b.Text)
		case model.BlockList:
			parts = append(parts, b.Items...)
		case model.BlockTable:
			for _, row := range b.Rows {
				for _, cell := range row {
					if cell != "" {
						parts = append(parts, cell)
					}
				}
			}
		case model.BlockImage:
			// 圖片不貢獻文字。
		}
	}
	return joinSpace(parts)
}

func joinSpace(parts []string) string {
	n := 0
	for _, p := range parts {
		n += len(p) + 1
	}
	b := make([]byte, 0, n)
	for i, p := range parts {
		if i > 0 {
			b = append(b, ' ')
		}
		b = append(b, p...)
	}
	return string(b)
}

type stored struct {
	Type    string     `json:"type"`
	Text    string     `json:"text,omitempty"`
	AssetID *uuid.UUID `json:"asset_id,omitempty"`
	Items   []string   `json:"items,omitempty"`
	Rows    [][]string `json:"rows,omitempty"`
}

// Encode 把整理過的區塊編成存進 products.description 的 JSON 陣列。圖片區塊的 Image.AssetID 必須已填（Confirm 之後）；
// alt 與 object key 存在 product_images，JSON 只放 asset_id。
func Encode(blocks []model.Block) ([]byte, error) {
	out := make([]stored, 0, len(blocks))
	for _, b := range blocks {
		s := stored{Type: string(b.Kind)}
		switch b.Kind {
		case model.BlockParagraph:
			s.Text = b.Text
		case model.BlockImage:
			if b.Image == nil || b.Image.AssetID == uuid.Nil() {
				return nil, fmt.Errorf("catalog: description image block without asset")
			}
			id := b.Image.AssetID
			s.AssetID = &id
		case model.BlockList:
			s.Items = b.Items
		case model.BlockTable:
			s.Rows = b.Rows
		default:
			return nil, fmt.Errorf("catalog: unknown block kind %q", b.Kind)
		}
		out = append(out, s)
	}
	return json.Marshal(out)
}

// Decode 讀回 products.description；圖片區塊的 alt 與 object key 從 images（以 asset_id 查）補上。
func Decode(raw []byte, images map[uuid.UUID]model.Image) ([]model.Block, error) {
	var in []stored
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("catalog: decode description: %w", err)
	}
	out := make([]model.Block, 0, len(in))
	for _, s := range in {
		b := model.Block{Kind: model.BlockKind(s.Type)}
		switch b.Kind {
		case model.BlockParagraph:
			b.Text = s.Text
		case model.BlockImage:
			if s.AssetID == nil {
				return nil, fmt.Errorf("catalog: stored image block without asset_id")
			}
			img, ok := images[*s.AssetID]
			if !ok {
				return nil, fmt.Errorf("catalog: stored image block points at missing asset %s", *s.AssetID)
			}
			b.Image = &img
		case model.BlockList:
			b.Items = s.Items
		case model.BlockTable:
			b.Rows = s.Rows
		default:
			return nil, fmt.Errorf("catalog: stored unknown block kind %q", s.Type)
		}
		out = append(out, b)
	}
	return out, nil
}
