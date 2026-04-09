# Validation Rules

Shared validation regexes and limits. Used in both production code and test assertions.

## Username

- Pattern: `^[a-z0-9_-]{1,30}$`
- Lowercase alphanumeric, hyphens, underscores
- 1-30 characters

## File Name

- Pattern: `^[a-z0-9_-]{1,245}\.txt$`
- Lowercase alphanumeric, hyphens, underscores + `.txt` extension
- Must end in `.txt`

## Folder Path

- Pattern: `^(/[a-z0-9_-]{1,10}){0,9}/$` or exactly `/`
- Each folder segment: 1-10 characters, lowercase alphanumeric + hyphens/underscores
- Trailing slash required
- Root is `/`
- Examples: `/`, `/blog/`, `/blog/2026/`

## Limits

| Limit | Value | HTTP status on violation |
|---|---|---|
| File size | ≤ 102400 bytes (100KB) | 413 Payload Too Large |
| Nesting depth | ≤ 10 levels | 400 Bad Request |
| Folder name length | ≤ 10 characters | 400 Bad Request |
| Subfolders per folder | ≤ 10 | 400 Bad Request |
| Files per folder | ≤ 100 | 400 Bad Request |

## HTTP Status Codes

| Code | When |
|---|---|
| 200 | Successful GET, login, signup |
| 201 | Successful PUT (page created/updated) |
| 400 | Bad path, bad filename, missing fields, invalid input |
| 401 | No token, invalid token |
| 403 | Token valid but wrong user (trying to write to another user's space) |
| 404 | Page not found, user not found |
| 409 | Username already taken on signup |
| 413 | File too large |
