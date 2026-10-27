package domain

import "errors"

var (
	ErrCustomerNotFound = errors.New("customer not found")
	ErrTagNotFound      = errors.New("tag not found")
	ErrTagAlreadyExists = errors.New("tag already exists")
	ErrTaskNotFound     = errors.New("marketing task not found")
)
