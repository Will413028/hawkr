package feature

type Deps struct{}

// Service 是子領域的 use case 容器。
type Service struct {
	d Deps
}

// New 建立子領域的 use case。
func New(d Deps) *Service {
	return &Service{d: d}
}
