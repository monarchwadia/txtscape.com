# npm README Mirrors Root README

## Context
The npm package page (npmjs.com/package/@txtscape/mcp) was showing a stale, minimal README with wrong tool names and no value prop. It looked unprofessional compared to the GitHub page.

## Decision
Keep the npm README (txtscape-mcp/npm/txtscape-mcp/README.md) essentially identical to the root README. Same sections, same copy, same order. Minor differences allowed:
- Title: `# txtscape` (not `# @txtscape/mcp`)
- Nav links: GitHub link instead of npm link
- License: simplified to just `MIT`

When the root README is updated, the npm README should be updated to match.

## Consequences
- One voice across GitHub and npm — visitors get the same pitch
- Less maintenance overhead than maintaining two different READMEs
- npm publish automatically picks up the latest copy
