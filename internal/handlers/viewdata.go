package handlers

import "github.com/gin-gonic/gin"

func withViewData(c *gin.Context, data gin.H) gin.H {
	if data == nil {
		data = gin.H{}
	}
	if _, exists := data["csrfToken"]; !exists {
		if token, ok := c.Get("csrfToken"); ok {
			if tokenStr, ok := token.(string); ok {
				data["csrfToken"] = tokenStr
			}
		}
	}
	return data
}
