package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// GitHub OIDC Middleware
	r.Use(GitHubOIDCMiddleware())

	// Serve HTML
	r.LoadHTMLFiles("frontend/index.html")
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})
	// // Static frontend
	// r.Static("/static", "./frontend")

	// Routes
	r.POST("/api/create", CreateRoleBinding)
	r.GET("/api/namespaces", ListNamespaces)
	r.POST("/api/cleanup", CleanupExpiredBindings)

	// Start scheduler in background
	go StartScheduler()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("Server running on port", port)
	r.Run(":" + port)
}
