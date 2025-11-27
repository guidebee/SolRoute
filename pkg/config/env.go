package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadEnv loads environment variables from .env file if it exists
func LoadEnv(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		// .env file is optional
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Only set if not already set in environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

// GetRPCEndpoints returns RPC endpoints from environment or default
func GetRPCEndpoints() []string {
	envEndpoints := os.Getenv("RPC_ENDPOINTS")
	if envEndpoints == "" {
		return nil
	}

	endpoints := strings.Split(envEndpoints, ",")
	result := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		trimmed := strings.TrimSpace(endpoint)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
