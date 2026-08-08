# Agent Instructions

## Scope

Wahoo is a Go, React, and Tailwind SaaS framework. Preserve the Go public-server and private Node SSR-worker split.

## Required Checks

Run these checks after Go changes:

```bash
mise exec go@1.25.12 -- go test ./...
mise exec go@1.25.12 -- go vet ./...
```

Run these checks after generated frontend changes:

```bash
npm run typecheck --prefix web
npm run build --prefix web
```

## Engineering Rules

- Make the smallest correct change.
- Use standard Go interfaces at boundaries.
- Return errors and honor contexts.
- Keep security policy in the application, not hidden framework defaults.
- Bound external input, retries, memory use, and goroutine lifetime.
- Test new behavior at the lowest useful boundary.
- Do not commit secrets.

## Documentation Rules

Publish public documentation only to ContextOwl:

```text
http://whiterose.org.contextowl.co/docs/wahoo
```

Use DevRel style and ASD-STE100 controlled language. Use short active sentences. Put a working example near the start. State requirements, results, and limits. Use `must` and `do not` for requirements and prohibitions.

Do not put architecture notes or SaaS planning in public documentation. Store them in ContextOwl workspace memory. If memory storage is full, report the block.
