package resolver

import (
	"fmt"
	"uuid"
)

func categoryID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-7000-8000-%012d", n))
}

func parentOf(n int) *uuid.UUID {
	id := categoryID(n)
	return &id
}
