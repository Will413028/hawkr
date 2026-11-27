package planner

import (
	"fmt"
	"uuid"
)

func categoryID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-7000-8000-%012d", n))
}
