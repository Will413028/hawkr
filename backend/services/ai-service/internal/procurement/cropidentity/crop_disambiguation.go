package cropidentity

import (
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
)

// DisambiguationResult 是一次解析的結果，只有兩種形狀：
//   - NeedsDisambiguation：Candidates 是要買家挑的分類，Selection 記著這張卡在問哪一層。
//   - Unavailable：沒有更多東西要問（品項已定到分類樹的某個節點，或分類樹找不到），
//     下一步是買家填單價；Selection.CategoryID 非空才表示分類已確認。
type DisambiguationResult struct {
	NeedsDisambiguation bool
	Candidates          []domain.CropDisambiguationOption
	Unavailable         bool

	Selection *domain.CropSelection `json:",omitempty"`
}

type DisambiguationUseCase struct {
	categories    cropCategoryReader
	categoryModel cropCategorySuggester
}

func NewDisambiguationUseCase(categories cropCategoryReader, model cropCategorySuggester) *DisambiguationUseCase {
	return &DisambiguationUseCase{categories: categories, categoryModel: model}
}
