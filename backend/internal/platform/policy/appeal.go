package policy

import "slices"

// AppealPath 是處分的受處分者可以走的申訴途徑；處分事件只帶這個值，文案由消費端決定。目前只有 AppealNone：買家沒有申訴流程（D25），
// 有流程時才加值。
type AppealPath string

// AppealNone 沒有申訴途徑。
const AppealNone AppealPath = "NONE"

// AppealPaths 回傳全部申訴途徑；每次回傳新的切片。
func AppealPaths() []AppealPath { return []AppealPath{AppealNone} }

// Valid 說明這是不是已知的申訴途徑；空字串不是。
func (p AppealPath) Valid() bool { return slices.Contains(AppealPaths(), p) }
