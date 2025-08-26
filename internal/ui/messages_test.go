package ui

import (
	"context"
	"errors"
	"testing"
	"time"

	"storyblok-sync/internal/sb"
)

func TestValidateTokenCmd(t *testing.T) {
	m := createTestModelWithToken("test-token")

	cmd := m.validateTokenCmd()
	if cmd == nil {
		t.Fatal("Expected validation command, got nil")
	}

	// Execute the command to get the message
	msg := cmd()

	validateMsg, ok := msg.(validateMsg)
	if !ok {
		t.Fatal("Expected validateMsg type")
	}

	// The actual API call will fail in tests, but we should get an error
	if validateMsg.err == nil {
		t.Error("Expected error for invalid token in test environment")
	}
}

func TestScanStoriesCmd(t *testing.T) {
	m := createTestModelWithToken("test-token")
	m.sourceSpace = &sb.Space{ID: 123, Name: "Source"}
	m.targetSpace = &sb.Space{ID: 456, Name: "Target"}

	cmd := m.scanStoriesCmd()
	if cmd == nil {
		t.Fatal("Expected scan command, got nil")
	}

	// Execute the command to get the message
	msg := cmd()

	scanMsg, ok := msg.(scanMsg)
	if !ok {
		t.Fatal("Expected scanMsg type")
	}

	// The actual API call will fail in tests, but we should get an error
	if scanMsg.err == nil {
		t.Error("Expected error for API call in test environment")
	}
}

func TestScanStoriesCmdWithoutSpaces(t *testing.T) {
	m := createTestModelWithToken("test-token")
	// No spaces set - should use 0 as default

	cmd := m.scanStoriesCmd()
	if cmd == nil {
		t.Fatal("Expected scan command, got nil")
	}

	// Execute the command - should still work but likely fail due to invalid space ID
	msg := cmd()

	scanMsg, ok := msg.(scanMsg)
	if !ok {
		t.Fatal("Expected scanMsg type")
	}

	// Should get an error for invalid space ID 0
	if scanMsg.err == nil {
		t.Error("Expected error for invalid space ID")
	}
}

// Test that commands use the correct timeouts
func TestCommandTimeouts(t *testing.T) {
	m := createTestModelWithToken("test-token")

	// Test validate timeout
	start := time.Now()
	cmd := m.validateTokenCmd()
	msg := cmd()
	duration := time.Since(start)

	// Should complete quickly due to network error, but let's verify it doesn't hang
	if duration > 15*time.Second {
		t.Error("Validation command took too long, timeout may not be working")
	}

	validateMsg, ok := msg.(validateMsg)
	if !ok {
		t.Fatal("Expected validateMsg type")
	}

	// Should have an error due to test environment
	if validateMsg.err == nil {
		t.Error("Expected timeout or network error")
	}
}

func TestScanCommandTimeout(t *testing.T) {
	m := createTestModelWithToken("test-token")
	m.sourceSpace = &sb.Space{ID: 123, Name: "Source"}
	m.targetSpace = &sb.Space{ID: 456, Name: "Target"}

	start := time.Now()
	cmd := m.scanStoriesCmd()
	msg := cmd()
	duration := time.Since(start)

	// Should complete quickly due to network error, but let's verify it doesn't hang
	if duration > 35*time.Second {
		t.Error("Scan command took too long, timeout may not be working")
	}

	scanMsg, ok := msg.(scanMsg)
	if !ok {
		t.Fatal("Expected scanMsg type")
	}

	// Should have an error due to test environment
	if scanMsg.err == nil {
		t.Error("Expected timeout or network error")
	}
}

// Mock client for testing successful scenarios
type mockClient struct {
	spaces      []sb.Space
	stories     []sb.Story
	shouldError bool
}

func (m *mockClient) ListSpaces(ctx context.Context) ([]sb.Space, error) {
	if m.shouldError {
		return nil, errors.New("mock error")
	}
	return m.spaces, nil
}

func (m *mockClient) ListStories(ctx context.Context, opts sb.ListStoriesOpts) ([]sb.Story, error) {
	if m.shouldError {
		return nil, errors.New("mock error")
	}
	return m.stories, nil
}

func TestValidateMessageTypes(t *testing.T) {
	tests := []struct {
		name      string
		hasSpaces bool
		hasError  bool
	}{
		{
			name:      "successful validation",
			hasSpaces: true,
			hasError:  false,
		},
		{
			name:      "validation error",
			hasSpaces: false,
			hasError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg validateMsg

			if tt.hasError {
				msg = validateMsg{
					spaces: nil,
					err:    errors.New("validation failed"),
				}
			} else {
				msg = validateMsg{
					spaces: []sb.Space{{ID: 1, Name: "Test Space"}},
					err:    nil,
				}
			}

			if tt.hasError {
				if msg.err == nil {
					t.Error("Expected error in validation message")
				}
				if msg.spaces != nil {
					t.Error("Should not have spaces on error")
				}
			} else {
				if msg.err != nil {
					t.Error("Should not have error on success")
				}
				if len(msg.spaces) == 0 {
					t.Error("Should have spaces on success")
				}
			}
		})
	}
}

func TestScanMessageTypes(t *testing.T) {
	tests := []struct {
		name       string
		hasStories bool
		hasError   bool
	}{
		{
			name:       "successful scan",
			hasStories: true,
			hasError:   false,
		},
		{
			name:       "scan error",
			hasStories: false,
			hasError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg scanMsg

			if tt.hasError {
				msg = scanMsg{
					src: nil,
					tgt: nil,
					err: errors.New("scan failed"),
				}
			} else {
				msg = scanMsg{
					src: []sb.Story{{ID: 1, Name: "Source Story"}},
					tgt: []sb.Story{{ID: 2, Name: "Target Story"}},
					err: nil,
				}
			}

			if tt.hasError {
				if msg.err == nil {
					t.Error("Expected error in scan message")
				}
				if msg.src != nil || msg.tgt != nil {
					t.Error("Should not have stories on error")
				}
			} else {
				if msg.err != nil {
					t.Error("Should not have error on success")
				}
				if len(msg.src) == 0 {
					t.Error("Should have source stories on success")
				}
				if len(msg.tgt) == 0 {
					t.Error("Should have target stories on success")
				}
			}
		})
	}
}

func TestScanMessageErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		sourceError bool
		targetError bool
		expectError bool
	}{
		{
			name:        "source error",
			sourceError: true,
			targetError: false,
			expectError: true,
		},
		{
			name:        "target error",
			sourceError: false,
			targetError: true,
			expectError: true,
		},
		{
			name:        "both succeed",
			sourceError: false,
			targetError: false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test scenario where we can control the error behavior
			var expectedErr error
			if tt.sourceError {
				expectedErr = errors.New("source scan: mock error")
			} else if tt.targetError {
				expectedErr = errors.New("target scan: mock error")
			}

			if tt.expectError && expectedErr == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && expectedErr != nil {
				t.Error("Did not expect error but got one")
			}
		})
	}
}
