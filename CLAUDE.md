This is a monorepo. Using `cd` is important.

Refer to Makefile if you have any doubt.

For catching syntax errors:

```
cd full_path_to_app && npx tsc --skipLibcheck
cd full_path_to_server && go vet ./...
```
