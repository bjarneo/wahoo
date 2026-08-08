# Wahoo Contributor Guide

Wahoo is a Go, React, and Tailwind SaaS framework. Go owns the public HTTP server. React renders pages through a private Node SSR worker.

## Commands

Run these commands from the repository root:

```bash
mise exec go@1.25.12 -- go test ./...
mise exec go@1.25.12 -- go vet ./...
mise exec go@1.25.12 -- make build VERSION=dev
sh -n install.sh
```

Run frontend checks from a generated application:

```bash
npm install --prefix web
npm run typecheck --prefix web
npm run build --prefix web
```

## Code Rules

- Keep framework packages small and independent.
- Use the standard Go HTTP interfaces at package boundaries.
- Pass `context.Context` as the first function argument.
- Return errors. Do not panic in library code.
- Define interfaces in the package that uses them.
- Bound request bodies, goroutine lifetimes, queues, and realtime clients.
- Do not add global mutable state.
- Add focused tests for new behavior.
- Run formatting, tests, and static checks before a commit.

## Documentation Rules

Public Wahoo documentation lives in the `cowl-wahoo` ContextOwl workspace:

```text
https://contextowl.co/docs/cowl-wahoo
```

Do not add public documentation pages to this Git repository. Update the ContextOwl articles instead.

Use DevRel documentation style:

- Start with the user outcome.
- Give a short, working command or code example early.
- State prerequisites and expected results.
- Link to the next task.
- Keep one subject in each page.

Use ASD-STE100 controlled language:

- Use short sentences.
- Use active voice.
- Use one instruction per step.
- Use approved technical names consistently.
- Use `must` for a requirement and `do not` for a prohibition.
- Avoid idioms, vague words, and marketing claims.
- Define a technical term before you use it when the reader can misunderstand it.

Only publish product-facing guides. Keep architecture notes, roadmap content, and SaaS planning in ContextOwl workspace memory. Do not publish those notes. If memory storage is unavailable, report the block instead of publishing internal content.

## Secret Rules

- Do not add tokens, passwords, private keys, or credentials to Git.
- Use environment references for MCP credentials.
- Do not print secrets in commands, logs, errors, or documentation.
