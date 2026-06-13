---
name: github-issue-flow
description: Use when given a GitHub issue number to fetch, implement, and complete. Triggers on "issue", "GitHub issue", "close issue", or any mention of an issue number like "#42". Automates the full workflow from reading the issue to opening a connected PR.
---

# GitHub Issue Flow

Use ONLY when the user references a GitHub issue number or asks to work on/close a GitHub issue.

## Overview

You will take a GitHub issue number, fetch its details, implement the required work following the project's TDD and architecture conventions, and open a pull request targeting the `production` branch.

**Critical fact: the default branch is `production`, not `main`.** Every branch and PR must reference `production`.

## Step 1: Fetch the issue

Retrieve the full issue details and structured metadata:

```bash
gh issue view <ISSUE_NUMBER>
gh issue view <ISSUE_NUMBER> --json labels,milestone,body,title
```

Parse the output carefully. Identify:
- The issue title and body (requirements, acceptance criteria).
- Any labels or milestone that constrain scope.
- Linked comments that add context or clarify requirements.

If the issue is unclear or missing critical details, ask the user before proceeding.

## Step 2: Create a branch

Create a new branch off `production`. The branch name must include the issue number and a short description:

```bash
git fetch origin
git checkout -b issue/<ISSUE_NUMBER>-<short-description> origin/production
```

Example: `issue/42-add-pull-sync`

## Step 3: Plan the work

Break the issue into discrete, testable units of work. Each unit must be small enough to:
- Write a failing test for (Red).
- Implement minimally to pass (Green).
- Commit independently.

List the planned units before starting implementation. If the issue spans multiple domain concepts or layers, plan them in dependency order (inner layers first: domain → application → infrastructure → CLI).

## Step 4: Implement iteratively

For each discrete unit of work, follow the red-green-refactor cycle:

### Red
Write the test first. It must fail. Confirm it fails by running:

```bash
go test ./...
```

If the test passes without implementation, the test is not valid. Rewrite it.

### Green
Write the minimal code to make the test pass. Nothing more.

### Refactor
Clean up the code while keeping all tests green. Run:

```bash
go test ./...
```

All tests must pass.

### Commit and push
After each unit is complete and tests are green:

```bash
git add -A
git commit -m "<descriptive subject>" -m "<detailed body explaining what and why>"
git push origin <current-branch>
```

**You MUST push after each commit.** Do not batch pushes. If a push fails, resolve the conflict and retry before moving to the next unit.

Commit message rules:
- Plain language subject line stating what the commit accomplishes.
- **No prefixes** like `feat:`, `fix:`, `refactor:`, etc.
- Use the commit body for detailed information.

## Step 5: Verify completeness

Before declaring the issue done:

1. Re-read the issue body and comments. Verify every acceptance criterion is met.
2. Run the full build and test suite:

```bash
go build
go test ./...
```

Both must succeed. If anything fails, fix it, commit, and push again.

## Step 6: Create the PR

Create a pull request targeting `production`:

```bash
gh pr create \
  --base production \
  --title "<issue title from the issue>" \
  --body "Closes #<ISSUE_NUMBER>

<summary of changes made, organized by the commits or units of work>"
```

If the issue has labels, include them on the PR:

```bash
gh pr create \
  --base production \
  --title "<issue title>" \
  --label "<label1>,<label2>" \
  --body "Closes #<ISSUE_NUMBER>

<summary of changes made>"
```

The PR body must:
- Reference the issue with `Closes #<ISSUE_NUMBER>` so it auto-closes on merge.
- Summarize the changes made, organized by feature or layer.

## Project conventions to follow

- **Language**: Go.
- **Architecture**: Clean Architecture / Onion + DDD. Inner layers (domain) must never import outer layers. Repository interfaces in `domain`, implementations in `infrastructure`.
- **Testing**: Table-driven tests preferred. Unit tests live alongside source (`*_test.go`). Use `testify/assert` or stdlib `testing` — be consistent within a package.
- **Default branch**: `production`.
- **Build**: `go build`.
- **Test**: `go test ./...`.
- **Commit messages**: No prefixes, plain language, body for details.
