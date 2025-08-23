package config

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var TOKEN_PATH = "./.sbrc"

func GetSbRc() (string, error) {
	rcFile, err := os.Open(TOKEN_PATH)
	if err != nil {
		fmt.Println("[file reader]: error finding rc:", err)
		return "", err
	}
	defer rcFile.Close()

	rcContent, err := io.ReadAll(rcFile)
	if err != nil {
		fmt.Println("[file reader]: error reading rc:", err)
		return "", err
	}

	parts := strings.SplitN(string(rcContent), "=", 2)
	if len(parts) != 2 {
		fmt.Println("[file reader]: invalid format in rc file")
		return "", err
	}
	token := strings.TrimSpace(parts[1])

	return token, nil
}
