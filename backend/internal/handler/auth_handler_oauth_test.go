package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRedirectOAuthSuccessForUserRejectsInactiveUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/wechat/callback", nil)

	h := &AuthHandler{}
	h.redirectOAuthSuccessForUser(c, "/auth/wechat/callback", "/dashboard", &service.User{
		ID:     7,
		Email:  "disabled@example.com",
		Status: service.StatusDisabled,
	})

	require.Equal(t, http.StatusFound, rec.Code)

	location := rec.Header().Get("Location")
	redirectURL, err := url.Parse(location)
	require.NoError(t, err)
	require.Equal(t, "/auth/wechat/callback", redirectURL.Path)

	fragment, err := url.ParseQuery(redirectURL.Fragment)
	require.NoError(t, err)
	require.Equal(t, "login_failed", fragment.Get("error"))
	require.Equal(t, "USER_NOT_ACTIVE", fragment.Get("error_message"))
	require.Equal(t, "user is not active", fragment.Get("error_description"))
	require.Empty(t, fragment.Get("access_token"))
}
