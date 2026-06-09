package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ServeMCP starts the agent as an MCP server over stdio.
func ServeMCP() error {
	s := server.NewMCPServer(
		"authgraph",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	registerMCPTools(s)

	return server.ServeStdio(s)
}

func registerMCPTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("check_permission",
			mcp.WithDescription("Check if a subject has a permission on a resource. Returns allowed/denied with optional traversal path."),
			mcp.WithString("subject_type", mcp.Description("Type of the subject (e.g., 'user', 'service', 'agent')"), mcp.Required()),
			mcp.WithString("subject_id", mcp.Description("ID of the subject (e.g., 'alice', 'agent-1')"), mcp.Required()),
			mcp.WithString("permission", mcp.Description("Permission to check (e.g., 'read', 'write', 'delete')"), mcp.Required()),
			mcp.WithString("resource_type", mcp.Description("Type of the resource (e.g., 'document', 'project')"), mcp.Required()),
			mcp.WithString("resource_id", mcp.Description("ID of the resource (e.g., 'readme', 'main')"), mcp.Required()),
			mcp.WithBoolean("trace", mcp.Description("Include traversal path in response")),
		),
		handleCheckPermission,
	)

	s.AddTool(
		mcp.NewTool("grant_permission",
			mcp.WithDescription("Grant a permission by creating a relationship tuple. Optionally set expiry."),
			mcp.WithString("subject_type", mcp.Description("Type of the subject"), mcp.Required()),
			mcp.WithString("subject_id", mcp.Description("ID of the subject"), mcp.Required()),
			mcp.WithString("relation", mcp.Description("Relation to grant (e.g., 'editor', 'viewer', 'owner')"), mcp.Required()),
			mcp.WithString("resource_type", mcp.Description("Type of the resource"), mcp.Required()),
			mcp.WithString("resource_id", mcp.Description("ID of the resource"), mcp.Required()),
			mcp.WithString("expires_at", mcp.Description("ISO 8601 expiration time (optional)")),
		),
		handleGrantPermission,
	)

	s.AddTool(
		mcp.NewTool("revoke_permission",
			mcp.WithDescription("Revoke a permission by deleting a relationship tuple."),
			mcp.WithString("subject_type", mcp.Description("Type of the subject"), mcp.Required()),
			mcp.WithString("subject_id", mcp.Description("ID of the subject"), mcp.Required()),
			mcp.WithString("relation", mcp.Description("Relation to revoke"), mcp.Required()),
			mcp.WithString("resource_type", mcp.Description("Type of the resource"), mcp.Required()),
			mcp.WithString("resource_id", mcp.Description("ID of the resource"), mcp.Required()),
		),
		handleRevokePermission,
	)

	s.AddTool(
		mcp.NewTool("list_resources",
			mcp.WithDescription("List all resources a subject can access with a given permission."),
			mcp.WithString("subject_type", mcp.Description("Type of the subject"), mcp.Required()),
			mcp.WithString("subject_id", mcp.Description("ID of the subject"), mcp.Required()),
			mcp.WithString("permission", mcp.Description("Permission to check"), mcp.Required()),
			mcp.WithString("resource_type", mcp.Description("Type of resources to list"), mcp.Required()),
		),
		handleListResources,
	)

	s.AddTool(
		mcp.NewTool("expand_access",
			mcp.WithDescription("List all subjects that have a given permission on a resource."),
			mcp.WithString("resource_type", mcp.Description("Type of the resource"), mcp.Required()),
			mcp.WithString("resource_id", mcp.Description("ID of the resource"), mcp.Required()),
			mcp.WithString("permission", mcp.Description("Permission to expand"), mcp.Required()),
		),
		handleExpandAccess,
	)

	s.AddTool(
		mcp.NewTool("push_schema",
			mcp.WithDescription("Push a permission schema to Authgraph."),
			mcp.WithString("schema", mcp.Description("YAML schema content defining types, relations, and permissions"), mcp.Required()),
		),
		handlePushSchema,
	)

	s.AddTool(
		mcp.NewTool("validate_schema",
			mcp.WithDescription("Validate a permission schema without applying it."),
			mcp.WithString("schema", mcp.Description("YAML schema content to validate"), mcp.Required()),
		),
		handleValidateSchema,
	)

	s.AddTool(
		mcp.NewTool("get_schema",
			mcp.WithDescription("Get the current permission schema for the organization."),
		),
		handleGetSchema,
	)

	s.AddTool(
		mcp.NewTool("test_permissions",
			mcp.WithDescription("Run permission test assertions. Pass tests as a JSON array of {name, subject, permission, resource, expected}."),
			mcp.WithString("tests_json", mcp.Description("JSON array of test cases: [{name, subject (type:id), permission, resource (type:id), expected (allowed/denied)}]"), mcp.Required()),
		),
		handleTestPermissions,
	)
}

// --- Tool Handlers ---

func handleCheckPermission(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	subjectType := request.GetString("subject_type", "")
	subjectID := request.GetString("subject_id", "")
	permission := request.GetString("permission", "")
	resourceType := request.GetString("resource_type", "")
	resourceID := request.GetString("resource_id", "")
	trace := request.GetBool("trace", false)

	body := map[string]interface{}{
		"subject":    map[string]string{"type": subjectType, "id": subjectID},
		"permission": permission,
		"resource":   map[string]string{"type": resourceType, "id": resourceID},
		"trace":      trace,
	}

	resp, err := apiRequest(ctx, http.MethodPost, "/v1/check", body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API error: %v", err)), nil
	}

	var result struct {
		Allowed  bool     `json:"allowed"`
		Path     []string `json:"path"`
		Duration string   `json:"duration"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Parse error: %v", err)), nil
	}

	var text string
	if result.Allowed {
		text = fmt.Sprintf("✓ ALLOWED — %s:%s can %s %s:%s", subjectType, subjectID, permission, resourceType, resourceID)
	} else {
		text = fmt.Sprintf("✗ DENIED — %s:%s cannot %s %s:%s", subjectType, subjectID, permission, resourceType, resourceID)
	}
	if len(result.Path) > 0 {
		text += "\n\nPath:\n"
		for _, step := range result.Path {
			text += fmt.Sprintf("  → %s\n", step)
		}
	}
	text += fmt.Sprintf("\nDuration: %s", result.Duration)

	return mcp.NewToolResultText(text), nil
}

func handleGrantPermission(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	subjectType := request.GetString("subject_type", "")
	subjectID := request.GetString("subject_id", "")
	relation := request.GetString("relation", "")
	resourceType := request.GetString("resource_type", "")
	resourceID := request.GetString("resource_id", "")
	expiresAt := request.GetString("expires_at", "")

	body := map[string]interface{}{
		"subject":  map[string]string{"type": subjectType, "id": subjectID},
		"relation": relation,
		"resource": map[string]string{"type": resourceType, "id": resourceID},
	}
	if expiresAt != "" {
		body["condition"] = map[string]string{"expires_at": expiresAt}
	}

	_, err := apiRequest(ctx, http.MethodPost, "/v1/tuples", body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API error: %v", err)), nil
	}

	text := fmt.Sprintf("✓ Granted %s on %s:%s to %s:%s", relation, resourceType, resourceID, subjectType, subjectID)
	if expiresAt != "" {
		text += fmt.Sprintf("\n  Expires: %s", expiresAt)
	}
	return mcp.NewToolResultText(text), nil
}

func handleRevokePermission(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	subjectType := request.GetString("subject_type", "")
	subjectID := request.GetString("subject_id", "")
	relation := request.GetString("relation", "")
	resourceType := request.GetString("resource_type", "")
	resourceID := request.GetString("resource_id", "")

	body := map[string]interface{}{
		"subject":  map[string]string{"type": subjectType, "id": subjectID},
		"relation": relation,
		"resource": map[string]string{"type": resourceType, "id": resourceID},
	}

	_, err := apiRequest(ctx, http.MethodDelete, "/v1/tuples", body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API error: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("✓ Revoked %s from %s:%s on %s:%s", relation, subjectType, subjectID, resourceType, resourceID)), nil
}

func handleListResources(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	subjectType := request.GetString("subject_type", "")
	subjectID := request.GetString("subject_id", "")
	permission := request.GetString("permission", "")
	resourceType := request.GetString("resource_type", "")

	body := map[string]interface{}{
		"subject":       map[string]string{"type": subjectType, "id": subjectID},
		"permission":    permission,
		"resource_type": resourceType,
	}

	resp, err := apiRequest(ctx, http.MethodPost, "/v1/query/list-resources", body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API error: %v", err)), nil
	}

	var result struct {
		Resources []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Parse error: %v", err)), nil
	}

	if len(result.Resources) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No %s resources found for %s:%s with %s permission.", resourceType, subjectType, subjectID, permission)), nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Resources %s:%s can %s:", subjectType, subjectID, permission))
	for _, r := range result.Resources {
		lines = append(lines, fmt.Sprintf("  • %s:%s", r.Type, r.ID))
	}
	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

func handleExpandAccess(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceType := request.GetString("resource_type", "")
	resourceID := request.GetString("resource_id", "")
	permission := request.GetString("permission", "")

	body := map[string]interface{}{
		"resource":   map[string]string{"type": resourceType, "id": resourceID},
		"permission": permission,
	}

	resp, err := apiRequest(ctx, http.MethodPost, "/v1/query/expand", body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API error: %v", err)), nil
	}

	var result struct {
		Subjects []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"subjects"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Parse error: %v", err)), nil
	}

	if len(result.Subjects) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No subjects have %s on %s:%s.", permission, resourceType, resourceID)), nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Subjects with %s on %s:%s:", permission, resourceType, resourceID))
	for _, s := range result.Subjects {
		lines = append(lines, fmt.Sprintf("  • %s:%s", s.Type, s.ID))
	}
	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

func handlePushSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	schema := request.GetString("schema", "")

	body := map[string]interface{}{
		"schema": schema,
		"format": "yaml",
	}

	resp, err := apiRequest(ctx, http.MethodPost, "/v1/schemas", body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API error: %v", err)), nil
	}

	var result struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Parse error: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("✓ Schema pushed successfully (version: %s)", result.Version)), nil
}

func handleValidateSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	schema := request.GetString("schema", "")

	body := map[string]interface{}{
		"schema": schema,
		"format": "yaml",
	}

	resp, err := apiRequest(ctx, http.MethodPost, "/v1/schemas/validate", body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API error: %v", err)), nil
	}

	var result struct {
		Valid  bool     `json:"valid"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Parse error: %v", err)), nil
	}

	if result.Valid {
		return mcp.NewToolResultText("✓ Schema is valid"), nil
	}

	text := "✗ Schema validation failed:\n"
	for _, e := range result.Errors {
		text += fmt.Sprintf("  • %s\n", e)
	}
	return mcp.NewToolResultText(text), nil
}

func handleGetSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp, err := apiRequest(ctx, http.MethodGet, "/v1/schemas", nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API error: %v", err)), nil
	}

	var result struct {
		Schema  string `json:"schema"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Parse error: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Schema (version %s):\n\n```yaml\n%s\n```", result.Version, result.Schema)), nil
}

func handleTestPermissions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	testsJSON := request.GetString("tests_json", "")

	var tests []struct {
		Name       string `json:"name"`
		Subject    string `json:"subject"`
		Permission string `json:"permission"`
		Resource   string `json:"resource"`
		Expected   string `json:"expected"`
	}
	if err := json.Unmarshal([]byte(testsJSON), &tests); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid tests_json: %v", err)), nil
	}

	passed := 0
	failed := 0
	var results []string

	for _, tc := range tests {
		subjectParts := strings.SplitN(tc.Subject, ":", 2)
		resourceParts := strings.SplitN(tc.Resource, ":", 2)
		if len(subjectParts) != 2 || len(resourceParts) != 2 {
			results = append(results, fmt.Sprintf("  ✗ %s — invalid format", tc.Name))
			failed++
			continue
		}

		body := map[string]interface{}{
			"subject":    map[string]string{"type": subjectParts[0], "id": subjectParts[1]},
			"permission": tc.Permission,
			"resource":   map[string]string{"type": resourceParts[0], "id": resourceParts[1]},
		}

		resp, err := apiRequest(ctx, http.MethodPost, "/v1/check", body)
		if err != nil {
			results = append(results, fmt.Sprintf("  ✗ %s — error: %v", tc.Name, err))
			failed++
			continue
		}

		var checkResult struct {
			Allowed bool `json:"allowed"`
		}
		if err := json.Unmarshal(resp, &checkResult); err != nil {
			results = append(results, fmt.Sprintf("  ✗ %s — parse error", tc.Name))
			failed++
			continue
		}

		expectAllowed := tc.Expected == "allowed" || tc.Expected == "allow" || tc.Expected == "true"
		if checkResult.Allowed == expectAllowed {
			results = append(results, fmt.Sprintf("  ✓ %s", tc.Name))
			passed++
		} else {
			actual := "denied"
			if checkResult.Allowed {
				actual = "allowed"
			}
			results = append(results, fmt.Sprintf("  ✗ %s — expected %s, got %s", tc.Name, tc.Expected, actual))
			failed++
		}
	}

	text := strings.Join(results, "\n")
	text += fmt.Sprintf("\n\n%d passed, %d failed, %d total", passed, failed, len(tests))

	if failed > 0 {
		return mcp.NewToolResultText("Permission tests failed:\n\n" + text), nil
	}
	return mcp.NewToolResultText("✓ All permission tests passed:\n\n" + text), nil
}

// ServeHTTP starts the agent as an HTTP server for Copilot Extension.
func ServeHTTP() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handleHealth)
	mux.HandleFunc("POST /agent", handleAgentRequest)

	fmt.Printf("authgraph-agent serving on :%s\n", port)
	return http.ListenAndServe(":"+port, mux)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok","agent":"authgraph"}`))
}
