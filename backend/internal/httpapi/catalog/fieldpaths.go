package catalog

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

func protoreflectName(s string) protoreflect.Name { return protoreflect.Name(s) }

// withPaths 讓錯誤裡的欄位 path 以自己對應自己（請求層檢查已經用請求的路徑）。
func withPaths(err error, paths ...string) error {
	m := make(rpc.FieldPaths, len(paths))
	for _, p := range paths {
		m[p] = p
	}
	return rpc.WithFieldPaths(err, m)
}
