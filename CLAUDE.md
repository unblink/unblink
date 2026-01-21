# Run scripts

This is a monorepo. When run scripts, use cd absolute_path

```
cd absolute_path && bun script.ts
cd absolute_path && go run script.go
```

# Makefile

There is a Makefile at root, can be used for generating protobuf types.

```
make generate
```

# Quick frontend type check

Do not use `bun run build`, rather use

```
cd ... && npx tsc --skipLibcheck
```

# Patch

There could be Go errors related to setConn with go2rtc. Patch them with.

```
make patch
```
