package database

import (
	"regexp"
	"strings"
	"toolkit/logger"
	"toolkit/models"
)

const contextLines = 2 // Number of lines of context to show before and after a comment

type commentRegex struct {
	Type    string
	Pattern *regexp.Regexp
}

var commentPatterns = []commentRegex{
	{Type: "HTML", Pattern: regexp.MustCompile(`<!--([\s\S]*?)-->`)},
	{Type: "JS_MULTI_LINE", Pattern: regexp.MustCompile(`/\*([\s\S]*?)\*/`)}, // Also CSS
	{Type: "JS_SINGLE_LINE", Pattern: regexp.MustCompile(`//(.*)`)},
	// CSS comments are covered by JS_MULTI_LINE, but we could add more specific ones if needed
}

// FindCommentsInText analyzes text content for comments.
func FindCommentsInText(content string) ([]models.CommentFinding, error) {
	var findings []models.CommentFinding
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	for i, line := range lines {
		lineNumber := i + 1

		// Check single-line comment patterns first for the current line
		jsSingleLineRegex := regexp.MustCompile(`//(.*)`)
		if matches := jsSingleLineRegex.FindStringSubmatchIndex(line); len(matches) > 0 {
			commentText := strings.TrimSpace(line[matches[2]:matches[3]])
			if commentText != "" {
				ctxBefore, ctxAfter := getContext(lines, i, 0)
				findings = append(findings, models.CommentFinding{
					LineNumber:    lineNumber,
					CommentText:   commentText,
					CommentType:   "JS_SINGLE_LINE",
					ContextBefore: ctxBefore,
					ContextAfter:  ctxAfter,
				})
				// Continue to check for other comment types on the same line if needed,
				// or assume one comment type per primary match point.
				// For simplicity, we'll allow multiple findings if patterns overlap.
			}
		}
	}

	// Check multi-line and HTML comments on the entire content
	// This is less precise for line numbers but captures multi-line blocks.
	multiLinePatterns := []commentRegex{
		{Type: "HTML", Pattern: regexp.MustCompile(`<!--([\s\S]*?)-->`)},
		{Type: "JS_MULTI_LINE/CSS", Pattern: regexp.MustCompile(`/\*([\s\S]*?)\*/`)},
	}

	for _, cr := range multiLinePatterns {
		matches := cr.Pattern.FindAllStringSubmatchIndex(content, -1)
		for _, matchIndices := range matches {
			if len(matchIndices) >= 4 { // Ensure we have the capturing group
				startOffset := matchIndices[0]
				// endOffset := matchIndices[1] // end of the whole match
				commentText := strings.TrimSpace(content[matchIndices[2]:matchIndices[3]])

				if commentText != "" {
					// Approximate line number by counting newlines before the match
					approxLineNumber := 1 + strings.Count(content[:startOffset], "\n")

					// For context, find the line index of the start of the comment
					lineIdx := approxLineNumber - 1
					ctxBefore, ctxAfter := getContext(lines, lineIdx, strings.Count(commentText, "\n"))

					findings = append(findings, models.CommentFinding{
						LineNumber:    approxLineNumber,
						CommentText:   commentText,
						CommentType:   cr.Type,
						ContextBefore: ctxBefore,
						ContextAfter:  ctxAfter,
					})
				}
			}
		}
	}

	if len(findings) == 0 {
		logger.Info("FindCommentsInText: No comments found.")
	} else {
		logger.Info("FindCommentsInText: Found %d comments.", len(findings))
	}
	return findings, nil
}

func getContext(lines []string, currentLineIndex int, commentLineSpan int) (before string, after string) {
	var beforeLines, afterLines []string

	startLine := currentLineIndex - contextLines
	if startLine < 0 {
		startLine = 0
	}
	for i := startLine; i < currentLineIndex; i++ {
		beforeLines = append(beforeLines, lines[i])
	}

	endLine := currentLineIndex + commentLineSpan + 1 // line after the comment block
	for i := endLine; i < endLine+contextLines && i < len(lines); i++ {
		afterLines = append(afterLines, lines[i])
	}

	return strings.Join(beforeLines, "\n"), strings.Join(afterLines, "\n")
}
