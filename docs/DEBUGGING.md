# Debugging Guide

## Enabling Debug Logging

To enable debug logging for the sync operations:

```bash
# Enable debug logging
export DEBUG=1
go run ./cmd/sbsync

# View logs in real-time (in another terminal)
tail -f debug.log
```

## Using Delve for Debugging

For step-by-step debugging with breakpoints:

```bash
# Start debugger in headless mode
dlv debug --headless --api-version=2 --listen=127.0.0.1:43000 .

# In another terminal, connect to the debugger
dlv connect 127.0.0.1:43000

# Set breakpoints and debug
(dlv) break internal/ui/sync.go:217  # Break in syncStoryContent
(dlv) break internal/ui/sync.go:142  # Break in syncFolder
(dlv) continue
```

## Debug Log Output

When debug logging is enabled, you'll see structured JSON logs with fields like `ts`, `level`, and `msg`. Sensitive values like tokens are redacted.

Verbose payload logging

- Flag: pass `-verbose` to `sbsync` to emit full story payloads for create/update/fetch calls.
- Env: alternatively set `SB_VERBOSE=1`.
- Default behavior (no verbose): payloads are summarized (sizes only) and large messages are truncated for readability.

- Sync operation start/completion for each item
- API calls and responses
- UUID management operations
- Retry attempts and rate limiting
- Parent folder resolution
- Translated slug processing

Example debug output:
```
Optimizing preflight with 15 items
Optimized to 12 items, sync order: folders first, then stories
Starting sync for item 0: folder-1 (folder: true)
Syncing folder: folder-1
Updated folder: folder-1
Sync completed for folder-1
Starting sync for item 1: folder-1/story-1 (folder: false)
Syncing story: folder-1/story-1
Created story: folder-1/story-1
Sync completed for folder-1/story-1
```

## Testing Sync Logic

To test the improved sync implementation:

1. **Set up two test spaces** in Storyblok
2. **Create test content** in the source space:
   - Some folders with nested structure
   - Stories inside folders
   - Stories with translated slugs
   - Published and unpublished content

3. **Run with debug logging**:
   ```bash
   DEBUG=1 go run ./cmd/sbsync -verbose
   ```

4. **Verify sync behavior**:
   - Folders are created before stories
   - Parent-child relationships are maintained
   - UUIDs are preserved
   - Published status is maintained
   - Translated slugs work correctly

## Key Improvements

The new sync implementation includes:

1. **Correct Sync Order**: Folders are always synced before stories, sorted by depth
2. **Complete Story Data**: Full content, UUID, translated slugs, and metadata
3. **UUID Management**: Maintains story identity across spaces
4. **Retry Logic**: Handles rate limiting and transient errors
5. **Proper API Usage**: Uses `with_slug` parameter and correct payload structure
6. **Translated Slugs**: Handles multi-language content properly
7. **Error Handling**: Comprehensive logging and graceful error recovery

## Troubleshooting

If you encounter issues:

1. Check the debug logs for error details
2. Verify API token has necessary permissions
3. Ensure target space exists and is accessible
4. Check rate limiting - the sync includes automatic retries
5. Verify folder structure exists in target before syncing stories

## Performance Notes

The sync is designed to be efficient:
- Folders are synced in dependency order (parents before children)
- API calls include retry logic for reliability
- Batch operations are optimized where possible
- Rate limiting is handled automatically
