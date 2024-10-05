package estimate

import (
	"bufio"
	"strings"
)

// Given a YAML file with manifests delimited by the string `---`,
// this function splits them & returns them as a slice of strings
func splitYAML(yamlContent string) []string {
	var manifests []string
	scanner := bufio.NewScanner(strings.NewReader(yamlContent))
	var sb strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "---" {
			manifests = append(manifests, sb.String())
			sb.Reset()
		} else {
			sb.WriteString(line + "\n")
		}
	}

	if sb.Len() > 0 {
		manifests = append(manifests, sb.String())
	}

	return manifests
}
