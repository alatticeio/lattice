package middleware

import "github.com/gin-gonic/gin"

// allowedOrigins is the whitelist of origins permitted by CORS.
// The wildcard "*" is intentionally omitted: when AllowCredentials is true,
// the CORS spec requires a specific origin, not "*".
var allowedOrigins = map[string]bool{
	"https://console.lattice.run": true,
	"http://localhost:5173":       true,
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Only set the specific origin if it's in the whitelist.
		// This is required because AllowCredentials=true prohibits "*".
		if allowedOrigins[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
