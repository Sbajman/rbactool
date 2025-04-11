package main

import (
	"github.com/coreos/go-oidc"
	"github.com/gin-gonic/gin"
)

var provider *oidc.Provider
var verifier *oidc.IDTokenVerifier

func GitHubOIDCMiddleware() gin.HandlerFunc {
	//ctx := context.Background()

	// provider, _ = oidc.NewProvider(ctx, "https://github.com/login/oauth")
	// config := &oauth2.Config{
	// 	ClientID:     "GITHUB_CLIENT_ID",
	// 	ClientSecret: "GITHUB_CLIENT_SECRET",
	// 	Endpoint:     provider.Endpoint(),
	// 	RedirectURL:  "http://localhost:8080/callback",
	// 	Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	// }
	// verifier = provider.Verifier(&oidc.Config{ClientID: "GITHUB_CLIENT_ID"})

	return func(c *gin.Context) {
		// This is a stub. Add session check or redirect to GitHub login if not logged in.
		// You can integrate a proper OIDC flow using gorilla/sessions and redirects.
		c.Next()
	}
}
