## 1.34.20251007

- Refactor: Replace local ANSI color constants/helpers with `github.com/bevelwork/quick_color`.
  - Use `qc.Color`, `qc.ColorizeBold`, `qc.AlternatingColor`, and `qc.Color*` constants.
  - Simplifies styling and centralizes terminal color logic across quick_* tools.
- Tooling: Added local replace for `quick_color` in `go.mod`.
- Maintenance: Formatted, tidied, and vetted code; tests run green for `quick_color`.


