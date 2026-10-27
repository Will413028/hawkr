package dto

type PaginatedResult[T any] struct {
	Items []T
	Total int64
}
