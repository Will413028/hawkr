package category

import "go.uber.org/fx"

var Module = fx.Module("category_client",
	fx.Provide(NewCategoryClient),
)
