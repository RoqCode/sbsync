package sync

import (
	"context"
	"strings"

	"storyblok-sync/internal/sb"
)

// TokenAPI is the minimal interface needed to resolve CDA tokens.
type TokenAPI interface {
	ListSpaceAPIKeys(ctx context.Context, spaceID int) ([]sb.APIKey, error)
}

// CDATokenInfo contains the selected CDA token and metadata.
type CDATokenInfo struct {
	Selected  string // The token to use; empty if unavailable
	Kind      string // "preview" or "public" or ""
	Public    string // Public token if present
	Preview   string // Preview token if present (preferred)
	Available bool   // True if Selected is non-empty
}

// ResolveCDAToken discovers the CDA token for a space. Prefers preview over public.
// Errors are treated as non-fatal; returns Available=false and no error on failure to fetch.
func ResolveCDAToken(ctx context.Context, api TokenAPI, spaceID int) (CDATokenInfo, error) {
	var info CDATokenInfo
	keys, err := api.ListSpaceAPIKeys(ctx, spaceID)
	if err != nil {
		// Non-fatal: return empty info, allow caller to fall back to MA reads.
		return info, nil
	}

	for _, k := range keys {
		acc := strings.ToLower(k.Access)
		switch acc {
		case "public":
			if info.Public == "" {
				info.Public = k.Token
			}
		case "private": // preview
			if info.Preview == "" {
				info.Preview = k.Token
			}
		}
	}

	if info.Preview != "" {
		info.Selected = info.Preview
		info.Kind = "preview"
		info.Available = true
		return info, nil
	}
	if info.Public != "" {
		info.Selected = info.Public
		info.Kind = "public"
		info.Available = true
		return info, nil
	}
	// No suitable token found; Available remains false.
	return info, nil
}
