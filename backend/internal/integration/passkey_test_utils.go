//go:build e2e

package integration

import "os"

const (
	backendBaseURLEnv   = "BASE_URL"
	frontendBaseURLEnv  = "FRONTEND_BASE_URL"
	endpointPrefixEnv   = "ENDPOINT_PREFIX"
	adminEmailEnv       = "ADMIN_EMAIL"
	adminPasswordEnv    = "ADMIN_PASSWORD"
	e2eUserEmailEnv     = "E2E_USER_EMAIL"
	e2eUserPasswordEnv  = "E2E_USER_PASSWORD"
	e2eUserNameEnv      = "E2E_USER_NAME"
	defaultBackendURL   = "http://localhost:8080"
	defaultFrontendURL  = "http://localhost:3000"
	defaultAdminEmail   = "admin@sub2api.local"
	defaultE2EUserEmail = "e2e-passkey-user@sub2api.local"
	defaultE2EUserPass  = "E2ePasskey@12345"
	defaultE2EUserName  = "e2e-passkey-user"
)

type e2eUserCredentials struct {
	Email    string
	Password string
	Username string
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func e2eBackendBaseURL() string {
	return getEnv(backendBaseURLEnv, defaultBackendURL)
}

func e2eFrontendBaseURL() string {
	return getEnv(frontendBaseURLEnv, defaultFrontendURL)
}

func e2eEndpointPrefix() string {
	return getEnv(endpointPrefixEnv, "")
}

func e2eAdminCredentials() e2eUserCredentials {
	return e2eUserCredentials{
		Email:    getEnv(adminEmailEnv, defaultAdminEmail),
		Password: getEnv(adminPasswordEnv, ""),
	}
}

func e2eSeedUserCredentials() e2eUserCredentials {
	return e2eUserCredentials{
		Email:    getEnv(e2eUserEmailEnv, defaultE2EUserEmail),
		Password: getEnv(e2eUserPasswordEnv, defaultE2EUserPass),
		Username: getEnv(e2eUserNameEnv, defaultE2EUserName),
	}
}
