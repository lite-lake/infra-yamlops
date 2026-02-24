package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func Confirm(message string, defaultYes bool) bool {
	prompt := message
	if defaultYes {
		prompt += " (Y/n): "
	} else {
		prompt += " (y/N): "
	}

	fmt.Print(prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return defaultYes
	}

	response = strings.TrimSpace(strings.ToLower(response))

	if response == "" {
		return defaultYes
	}

	return response == "y" || response == "yes"
}

func ConfirmWithDefault(message string) bool {
	return Confirm(message, false)
}
