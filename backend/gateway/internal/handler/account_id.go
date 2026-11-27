package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func getAccountID(c *gin.Context) (int64, bool) {
	accountIDRaw, exists := c.Get("accountId")
	if !exists {
		return 0, false
	}
	accountID, ok := accountIDRaw.(int64)
	return accountID, ok
}

func parseAccountID(v interface{}) int64 {
	switch id := v.(type) {
	case int64:
		return id
	case float64:
		return int64(id)
	case string:
		n, _ := strconv.ParseInt(id, 10, 64)
		return n
	default:
		return 0
	}
}
