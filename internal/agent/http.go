package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type copilotRequest struct {
	Messages []chatMessage `json:"messages"`
}

func handleAgentRequest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req copilotRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Get the last user message
	var userMessage string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			userMessage = req.Messages[i].Content
			break
		}
	}

	if userMessage == "" {
		writeSSE(w, "I didn't receive a message. How can I help with permissions?")
		return
	}

	ctx := r.Context()
	content := strings.ToLower(userMessage)

	// Route to handler based on intent
	switch {
	case strings.Contains(content, "check") || strings.Contains(content, "can ") || strings.Contains(content, "allowed"):
		handleHTTPCheck(ctx, w, userMessage)
	case strings.Contains(content, "grant") || strings.Contains(content, "give access"):
		handleHTTPGrant(ctx, w, userMessage)
	case strings.Contains(content, "revoke") || strings.Contains(content, "remove access"):
		handleHTTPRevoke(ctx, w, userMessage)
	case strings.Contains(content, "who has") || strings.Contains(content, "who can") || strings.Contains(content, "expand"):
		handleHTTPExpand(ctx, w, userMessage)
	case strings.Contains(content, "what can") || strings.Contains(content, "list"):
		handleHTTPList(w)
	case strings.Contains(content, "schema"):
		handleHTTPSchema(ctx, w, userMessage)
	default:
		writeSSE(w, `I'm the Authgraph permissions agent. I can help you with:

• **Check permissions** — "Can user:alice read document:readme?"
• **Grant access** — "Grant editor on project:main to user:bob"
• **Revoke access** — "Revoke viewer from user:eve on document:secret"
• **List resources** — "What can user:alice access?"
• **Expand access** — "Who has access to document:readme?"
• **Schema** — "Show me the current schema"

What would you like to do?`)
	}
}

func handleHTTPCheck(ctx context.Context, w http.ResponseWriter, message string) {
	parsed := parseEntityPair(message)
	if parsed == nil {
		writeSSE(w, "I couldn't parse the permission check. Use format: `Can <type>:<id> <permission> <type>:<id>?`")
		return
	}

	body := map[string]interface{}{
		"subject":    map[string]string{"type": parsed.subjectType, "id": parsed.subjectID},
		"permission": parsed.permission,
		"resource":   map[string]string{"type": parsed.resourceType, "id": parsed.resourceID},
		"trace":      true,
	}

	resp, err := apiRequest(ctx, http.MethodPost, "/v1/check", body)
	if err != nil {
		writeSSE(w, fmt.Sprintf("Error: %v", err))
		return
	}

	var result struct {
		Allowed  bool     `json:"allowed"`
		Path     []string `json:"path"`
		Duration string   `json:"duration"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		writeSSE(w, fmt.Sprintf("Error parsing response: %v", err))
		return
	}

	var text string
	if result.Allowed {
		text = fmt.Sprintf("✅ **ALLOWED** — `%s:%s` can `%s` `%s:%s`", parsed.subjectType, parsed.subjectID, parsed.permission, parsed.resourceType, parsed.resourceID)
	} else {
		text = fmt.Sprintf("❌ **DENIED** — `%s:%s` cannot `%s` `%s:%s`", parsed.subjectType, parsed.subjectID, parsed.permission, parsed.resourceType, parsed.resourceID)
	}
	if len(result.Path) > 0 {
		text += "\n\n**Path:**\n"
		for _, step := range result.Path {
			text += fmt.Sprintf("→ %s\n", step)
		}
	}
	text += fmt.Sprintf("\n*Duration: %s*", result.Duration)
	writeSSE(w, text)
}

func handleHTTPGrant(ctx context.Context, w http.ResponseWriter, message string) {
	parsed := parseGrantRevoke(message)
	if parsed == nil {
		writeSSE(w, "I couldn't parse the grant. Use format: `Grant <relation> on <type>:<id> to <type>:<id>`")
		return
	}

	body := map[string]interface{}{
		"subject":  map[string]string{"type": parsed.subjectType, "id": parsed.subjectID},
		"relation": parsed.relation,
		"resource": map[string]string{"type": parsed.resourceType, "id": parsed.resourceID},
	}

	_, err := apiRequest(ctx, http.MethodPost, "/v1/tuples", body)
	if err != nil {
		writeSSE(w, fmt.Sprintf("Error: %v", err))
		return
	}

	writeSSE(w, fmt.Sprintf("✅ Granted `%s` on `%s:%s` to `%s:%s`", parsed.relation, parsed.resourceType, parsed.resourceID, parsed.subjectType, parsed.subjectID))
}

func handleHTTPRevoke(ctx context.Context, w http.ResponseWriter, message string) {
	parsed := parseGrantRevoke(message)
	if parsed == nil {
		writeSSE(w, "I couldn't parse the revoke. Use format: `Revoke <relation> from <type>:<id> on <type>:<id>`")
		return
	}

	body := map[string]interface{}{
		"subject":  map[string]string{"type": parsed.subjectType, "id": parsed.subjectID},
		"relation": parsed.relation,
		"resource": map[string]string{"type": parsed.resourceType, "id": parsed.resourceID},
	}

	_, err := apiRequest(ctx, http.MethodDelete, "/v1/tuples", body)
	if err != nil {
		writeSSE(w, fmt.Sprintf("Error: %v", err))
		return
	}

	writeSSE(w, fmt.Sprintf("✅ Revoked `%s` from `%s:%s` on `%s:%s`", parsed.relation, parsed.subjectType, parsed.subjectID, parsed.resourceType, parsed.resourceID))
}

func handleHTTPExpand(ctx context.Context, w http.ResponseWriter, message string) {
	parsed := parseExpandQuery(message)
	if parsed == nil {
		writeSSE(w, "I couldn't parse the expand. Use format: `Who has <permission> on <type>:<id>?`")
		return
	}

	body := map[string]interface{}{
		"resource":   map[string]string{"type": parsed.resourceType, "id": parsed.resourceID},
		"permission": parsed.permission,
	}

	resp, err := apiRequest(ctx, http.MethodPost, "/v1/query/expand", body)
	if err != nil {
		writeSSE(w, fmt.Sprintf("Error: %v", err))
		return
	}

	var result struct {
		Subjects []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"subjects"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		writeSSE(w, fmt.Sprintf("Error: %v", err))
		return
	}

	if len(result.Subjects) == 0 {
		writeSSE(w, fmt.Sprintf("No subjects have `%s` on `%s:%s`.", parsed.permission, parsed.resourceType, parsed.resourceID))
		return
	}

	text := fmt.Sprintf("**Subjects with `%s` on `%s:%s`:**\n\n", parsed.permission, parsed.resourceType, parsed.resourceID)
	for _, s := range result.Subjects {
		text += fmt.Sprintf("• `%s:%s`\n", s.Type, s.ID)
	}
	writeSSE(w, text)
}

func handleHTTPList(w http.ResponseWriter) {
	writeSSE(w, "Use: `What can <type>:<id> <permission> <resource_type>?`\n\nExample: `What can user:alice read document?`")
}

func handleHTTPSchema(ctx context.Context, w http.ResponseWriter, message string) {
	resp, err := apiRequest(ctx, http.MethodGet, "/v1/schemas", nil)
	if err != nil {
		writeSSE(w, fmt.Sprintf("Error: %v", err))
		return
	}

	var result struct {
		Schema  string `json:"schema"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		writeSSE(w, fmt.Sprintf("Error: %v", err))
		return
	}

	writeSSE(w, fmt.Sprintf("**Schema** (version %s):\n\n```yaml\n%s\n```", result.Version, result.Schema))
}

// --- Response Helper ---

func writeSSE(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	fmt.Fprintf(w, "event: copilot_message\ndata: %s\n\n", toJSON(map[string]string{"content": text}))
	fmt.Fprintf(w, "event: done\ndata: {}\n\n")

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func toJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// --- Intent Parsers ---

type checkParsed struct {
	subjectType  string
	subjectID    string
	permission   string
	resourceType string
	resourceID   string
}

type grantParsed struct {
	subjectType  string
	subjectID    string
	relation     string
	resourceType string
	resourceID   string
}

type expandParsed struct {
	resourceType string
	resourceID   string
	permission   string
}

func parseEntityPair(message string) *checkParsed {
	// Pattern: "user:alice read document:readme"
	words := strings.Fields(message)
	for i := 0; i < len(words)-2; i++ {
		subject := parseEntity(words[i])
		if subject == nil {
			continue
		}
		// Next non-entity word is permission, then resource
		for j := i + 1; j < len(words)-1; j++ {
			if parseEntity(words[j]) != nil {
				continue
			}
			permission := strings.Trim(words[j], "?!.,")
			for k := j + 1; k < len(words); k++ {
				resource := parseEntity(words[k])
				if resource != nil {
					return &checkParsed{
						subjectType:  subject[0],
						subjectID:    subject[1],
						permission:   permission,
						resourceType: resource[0],
						resourceID:   resource[1],
					}
				}
			}
			break
		}
	}
	return nil
}

func parseGrantRevoke(message string) *grantParsed {
	words := strings.Fields(message)
	var entities [][]string
	var nonEntities []string

	for _, w := range words {
		clean := strings.Trim(w, "?!.,")
		e := parseEntity(clean)
		if e != nil {
			entities = append(entities, e)
		} else if clean != "" && !isStopWord(clean) {
			nonEntities = append(nonEntities, clean)
		}
	}

	if len(entities) < 2 || len(nonEntities) < 1 {
		return nil
	}

	// First entity after "grant/revoke" keyword is typically the relation target
	// Heuristic: first non-stop word after grant/revoke is the relation
	relation := ""
	for _, w := range nonEntities {
		if w != "grant" && w != "revoke" && w != "on" && w != "to" && w != "from" {
			relation = w
			break
		}
	}
	if relation == "" {
		return nil
	}

	return &grantParsed{
		subjectType:  entities[1][0],
		subjectID:    entities[1][1],
		relation:     relation,
		resourceType: entities[0][0],
		resourceID:   entities[0][1],
	}
}

func parseExpandQuery(message string) *expandParsed {
	words := strings.Fields(message)
	var permission string
	var entity []string

	for _, w := range words {
		clean := strings.Trim(w, "?!.,")
		e := parseEntity(clean)
		if e != nil {
			entity = e
		} else if !isStopWord(clean) && clean != "who" && clean != "has" && clean != "can" {
			if permission == "" {
				permission = clean
			}
		}
	}

	if entity == nil || permission == "" {
		return nil
	}

	return &expandParsed{
		resourceType: entity[0],
		resourceID:   entity[1],
		permission:   permission,
	}
}

func parseEntity(s string) []string {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" && !strings.Contains(parts[0], " ") {
		return parts
	}
	return nil
}

func isStopWord(w string) bool {
	switch strings.ToLower(w) {
	case "the", "a", "an", "on", "to", "from", "for", "of", "in", "at", "by", "is", "are", "was",
		"can", "has", "have", "do", "does", "access", "permission", "check", "grant", "revoke":
		return true
	}
	return false
}
