---
status: complete
priority: p1
issue_id: "036"
tags: [code-review, scanner, data-bug, false-positive]
dependencies: []
---

# discord.js Listed in Both Blocklist AND Popular Packages

## Problem Statement

`discord.js` appears in both `blocklist_packages` (line 193) and `popular_packages` (line 377) in `default_patterns.yaml`. Since the blocklist check runs first in the pattern stage (`checkDepBlocklist`), the legitimate npm package `discord.js` will always be blocked. This is a false positive -- `discord.js` is a real, widely-used npm package with 25M+ weekly downloads.

The malicious package was likely a typosquat of `discord.js` (e.g., `discordjs`, `discord-js`), not `discord.js` itself.

## Findings

- `default_patterns.yaml:193` -- `discord.js` in `blocklist_packages`
- `default_patterns.yaml:377` -- `discord.js` in `popular_packages`
- `stage_patterns.go:checkDepBlocklist` -- blocklist check runs first, always blocks
- Pattern Recognition Specialist: Finding 7a

## Proposed Solutions

### Option A: Remove discord.js from Blocklist (Recommended)
Remove `discord.js` from `blocklist_packages`. It is a legitimate package. Add the actual typosquats instead (e.g., `discordjs`, `discord-js`, `disc0rd.js`).

- **Pros:** Fixes false positive, adds real typosquats
- **Cons:** Need to research actual malicious variants
- **Effort:** Small
- **Risk:** Low

## Acceptance Criteria

- [ ] `discord.js` removed from `blocklist_packages`
- [ ] `discord.js` remains in `popular_packages` (for typosquatting detection)
- [ ] Known typosquats of discord.js added to blocklist
- [ ] No test regressions
