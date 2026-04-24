package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"

	"github.com/gin-gonic/gin"
)

// RegisterCustomRoutes registers external integration endpoints that are kept
// isolated from the versioned admin/user APIs.
func RegisterCustomRoutes(r *gin.Engine, h *handler.Handlers) {
	if h == nil || h.CardsIssue == nil {
		return
	}
	custom := r.Group("/api/custom")
	{
		cards := custom.Group("/cards")
		{
			cards.POST("/issue", h.CardsIssue.Issue)
		}
	}
}
