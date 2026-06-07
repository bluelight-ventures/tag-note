package mcpserver

import (
	"fmt"
	"slices"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func userIDFromToken(tokenInfo *sdkauth.TokenInfo, requiredScope string) (string, error) {
	if tokenInfo == nil || tokenInfo.UserID == "" {
		return "", fmt.Errorf("authenticated MCP token is required")
	}
	if requiredScope != "" && !slices.Contains(tokenInfo.Scopes, requiredScope) {
		return "", fmt.Errorf("missing required OAuth scope %q", requiredScope)
	}
	return tokenInfo.UserID, nil
}
