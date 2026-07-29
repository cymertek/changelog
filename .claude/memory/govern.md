---
name: govah-to-gchangelog-rename
description: Project renamed from govah to gchangelog in July 2026
metadata:
  type: project
  date: 2026-07-26
---

Project was renamed from "govah" to "gchangelog". This affected:
- Module path: github.com/cymertek/gchangelog (renamed from changelog to gchangelog for install clarity)
- Binary name: govah → gchangelog  
- All test variables: GOVAH → GCHANGELOG
- CLI strings: 'govah init' → 'gchangelog init', etc.
- Directory: /workdir/govah → /workdir/gchangelog

All 129 parity tests pass with identical results after rename (0 failed, 12 warnings).
