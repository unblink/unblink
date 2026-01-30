This is a monorepo.

Refer to Makefile if you have any doubt.

For catching syntax errors:

```
make typecheck
```

# Important

You must use `cd`. Your tool call should look like this:
Bash(command="cd $ROOT/app && npx tsc --skipLibCheck 2>&1")

If you run npx in the root, it will give errors because there is no tsconfig.json there.

# Never start the app yourself

If you need testing, always ask user to start it themselves. Do not use `go run` or `bun dev`.
