package middleware

import (
	"github.com/rs/cors"
)

type Middleware struct {
	Cors *cors.Cors
}

func NewMiddleware() *Middleware {
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		// Enable Debugging for testing, consider disabling in production
		Debug: false,
	})
	return &Middleware{
		Cors: c,
	}
}
