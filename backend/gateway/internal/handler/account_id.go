package handler

import "github.com/gin-gonic/gin"

// getAccountID extracts the authenticated account ID from the request context.
func getAccountID(c *gin.Context) (int64, bool) {
	accountIDRaw, exists := c.Get("accountId")
	if !exists {
		return 0, false
	}
	accountID, ok := accountIDRaw.(int64)
	return accountID, ok
}
