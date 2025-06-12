package models

// CommentFinding represents a comment found in source code.
type CommentFinding struct {
	LineNumber    int    `json:"line_number"`    // Approximate starting line number of the comment
	CommentText   string `json:"comment_text"`   // The content of the comment
	CommentType   string `json:"comment_type"`   // Type of comment (e.g., "HTML", "JS_SINGLE_LINE", "JS_MULTI_LINE", "CSS")
	ContextBefore string `json:"context_before"` // A few lines of context before the comment
	ContextAfter  string `json:"context_after"`  // A few lines of context after the comment
}
