// Gin app for the versaDeploy e2e.
package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "versa-e2e gin %s\n", os.Getenv("APP_GREETING"))
	})
	r.Run("127.0.0.1:" + os.Getenv("PORT"))
}
