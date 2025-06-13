package models

// CommentFinding represents a comment found in text content.
type CommentFinding struct {
	LineNumber    int    `json:"lineNumber"`
	CommentText   string `json:"commentText"`
	CommentType   string `json:"commentType"`
	ContextBefore string `json:"contextBefore"`
	ContextAfter  string `json:"contextAfter"`
}
