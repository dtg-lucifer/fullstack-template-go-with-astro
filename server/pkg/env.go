package pkg

import "os"

// GetEnv returns the value of the environment variable key, or def if not set.
func GetEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}
