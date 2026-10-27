package sourcemarket

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

var (
	ErrDigestMismatch = errors.New("sourcemarket: registry sha256 與 manifest 不符")

	ErrUnknownMarket = errors.New("sourcemarket: 上游地名不在 registry 中")
)

type manifest struct {
	SHA256   string `json:"sha256"`
	RowCount int    `json:"row_count"`
}

type Registry struct {
	label   string
	version string
	byName  map[string]string
}

func (r Registry) Version() string { return r.version }

func (r Registry) Len() int { return len(r.byName) }

func (r Registry) Resolve(sourceMarketName string) (string, error) {
	code, ok := r.byName[strings.TrimSpace(sourceMarketName)]
	if !ok {
		return "", fmt.Errorf("%w（%s）: %q", ErrUnknownMarket, r.label, sourceMarketName)
	}
	return code, nil
}

func Load(label, csvPath, manifestPath string) (Registry, error) {
	csvBytes, err := os.ReadFile(csvPath)
	if err != nil {
		return Registry{}, fmt.Errorf("讀取 %s registry %s 失敗: %w", label, csvPath, err)
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return Registry{}, fmt.Errorf("讀取 manifest %s 失敗: %w", manifestPath, err)
	}
	var mf manifest
	if err := json.Unmarshal(manifestBytes, &mf); err != nil {
		return Registry{}, fmt.Errorf("解析 manifest %s 失敗: %w", manifestPath, err)
	}
	sum := sha256.Sum256(csvBytes)
	if digest := hex.EncodeToString(sum[:]); digest != mf.SHA256 {
		return Registry{}, fmt.Errorf("%w（%s）：實際 %s，manifest 記錄 %s", ErrDigestMismatch, label, digest, mf.SHA256)
	}

	rows, err := csv.NewReader(bytes.NewReader(csvBytes)).ReadAll()
	if err != nil {
		return Registry{}, fmt.Errorf("解析 %s registry CSV 失敗: %w", label, err)
	}
	if len(rows) < 2 {
		return Registry{}, fmt.Errorf("sourcemarket: %s registry 沒有任何資料列", label)
	}
	header := rows[0]
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	for _, col := range []string{"registry_version", "market_code", "source_market_name"} {
		if _, ok := idx[col]; !ok {
			return Registry{}, fmt.Errorf("sourcemarket: %s registry 缺少欄位 %q", label, col)
		}
	}

	out := Registry{label: label, byName: make(map[string]string, len(rows)-1)}
	seenCode := make(map[string]string, len(rows)-1)
	for _, row := range rows[1:] {
		name := strings.TrimSpace(row[idx["source_market_name"]])
		code := strings.TrimSpace(row[idx["market_code"]])
		ver := strings.TrimSpace(row[idx["registry_version"]])
		if name == "" || code == "" {
			return Registry{}, fmt.Errorf("sourcemarket: %s 有空的地名或代碼（row=%v）", label, row)
		}
		if prev, dup := out.byName[name]; dup {
			return Registry{}, fmt.Errorf("sourcemarket: %s 地名 %q 重複（已對到 %s）", label, name, prev)
		}

		if prev, dup := seenCode[code]; dup {
			return Registry{}, fmt.Errorf("sourcemarket: %s 代碼 %s 重複（已用於 %q）", label, code, prev)
		}
		seenCode[code] = name
		out.byName[name] = code
		if out.version == "" {
			out.version = ver
		} else if out.version != ver {
			return Registry{}, fmt.Errorf("sourcemarket: %s registry_version 不一致（%s vs %s）", label, out.version, ver)
		}
	}
	if mf.RowCount != len(out.byName) {
		return Registry{}, fmt.Errorf("sourcemarket: %s manifest 記錄 %d 列，實際 %d 列", label, mf.RowCount, len(out.byName))
	}
	return out, nil
}
