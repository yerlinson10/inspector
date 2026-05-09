package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func DocsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "docs.html", withViewData(c, gin.H{
		"ContentTemplate": "docs_content",
		"title":           "Documentation",
	}))
}
