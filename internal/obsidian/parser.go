package obsidian

import (
	"regexp"
)

// NoteMetadata representa os dados extraídos de uma nota.
type NoteMetadata struct {
	Title string
	Links []string
	Tags  []string
}

// ParseNote extrai links e ralações de uma nota Obsidian.
func ParseNote(content string) *NoteMetadata {
	meta := &NoteMetadata{}

	// Regex para extrair links do Wiki [[links]]
	reLinks := regexp.MustCompile(`\[\[(.*?)\]\]`)
	matches := reLinks.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			meta.Links = append(meta.Links, match[1])
		}
	}

	// Regex simples para extrair #tags
	reTags := regexp.MustCompile(`#([a-zA-Z0-9_\-]+)`)
	tagMatches := reTags.FindAllStringSubmatch(content, -1)

	for _, match := range tagMatches {
		if len(match) > 1 {
			meta.Tags = append(meta.Tags, match[1])
		}
	}

	return meta
}
