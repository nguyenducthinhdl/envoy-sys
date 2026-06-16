# envoyviz

Parse and compare Envoy configuration from JSON or YAML files (admin config-dump or native `static_resources` shape). Shows listeners, HTTP filter chains, routes (match → cluster → destination endpoints), and clusters.

Independent Go module — can be extracted to its own repository without code changes.

## Install

```bash
cd visualization
go install ./cmd/envoyviz
```

## Usage

```bash
# Parse a single file
envoyviz parse testdata/config-dump-1.json
envoyviz parse testdata/bootstrap.yaml

# Parse a folder (merges all .json/.yaml/.yml files)
envoyviz parse testdata/config-1/

# Diff file or folder inputs
envoyviz diff testdata/config-dump-1.json testdata/config-dump-2.json
envoyviz diff testdata/config-1/ testdata/config-2/
envoyviz diff --left config-1 --right config-2 testdata/config-1/ testdata/config-2/

# Stdin (parse only)
envoyviz parse - --format yaml < testdata/bootstrap.yaml
```

Exit codes:
- `parse`: `0` on success
- `diff`: `0` if identical, `1` if differences found

## Input formats

| Shape | Marker |
|-------|--------|
| Admin config dump | `{ "configs": [ ... ] }` |
| Native Envoy config | `static_resources`, or top-level `listeners` / `clusters` |

Each CLI path may be a **file** or **folder**. Folders are scanned for `.json`, `.yaml`, and `.yml` files (use `--recursive` for subdirectories). Multiple files in a folder are merged by resource name; later files win on collision.

## Library API

```go
cfg, err := envoyviz.ParsePath("testdata/config-dump-1.json", envoyviz.ParseOptions{})
result := envoyviz.Compare(left, right)
envoyviz.RenderDiff(os.Stdout, left, right, result, envoyviz.RenderOptions{})
```

## Extract to standalone repo

1. Copy or `git subtree split` the `visualization/` directory
2. Update `module` path in `go.mod` if needed
3. Run `go test ./...`

## Test

```bash
go test ./...
```
