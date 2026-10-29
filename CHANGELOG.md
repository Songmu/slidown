# Changelog

## [v0.0.12](https://github.com/Songmu/slidown/compare/v0.0.11...v0.0.12) - 2026-10-29

### Other Changes
- Deduplicate SVG fallback test execution by @Songmu in https://github.com/Songmu/slidown/pull/124
- Separate root namespace collection from insertion to clarify shape merging by @Songmu in https://github.com/Songmu/slidown/pull/125
- refactor: isolate SVG intrinsic image sizing by @Songmu in https://github.com/Songmu/slidown/pull/128
- Separate gradient stop interpretation from offset ordering by @Songmu in https://github.com/Songmu/slidown/pull/126
- Separate SVG symbol expansion from use reference handling by @Songmu in https://github.com/Songmu/slidown/pull/127
- Centralize presentation-order slide fallback by @Songmu in https://github.com/Songmu/slidown/pull/130
- Characterize SVG curves and serialized PPTX paths by @Songmu in https://github.com/Songmu/slidown/pull/131
- Consolidate placeholder classification scenarios and assertions by @Songmu in https://github.com/Songmu/slidown/pull/134
- Separate SVG text run bounds from shape emission by @Songmu in https://github.com/Songmu/slidown/pull/133
- test: verify image JSON roundtrips and populated receiver replacement by @Songmu in https://github.com/Songmu/slidown/pull/132
- refactor(pptx): separate default style dependency retention by @Songmu in https://github.com/Songmu/slidown/pull/135
- refactor(render): share body placeholder emission by @Songmu in https://github.com/Songmu/slidown/pull/136
- test: exercise CEL template evaluation contracts by @Songmu in https://github.com/Songmu/slidown/pull/138
- test: cover theme color resolution precedence by @Songmu in https://github.com/Songmu/slidown/pull/139
- test: cover fingerprint equivalence boundaries by @Songmu in https://github.com/Songmu/slidown/pull/137

## [v0.0.11](https://github.com/Songmu/slidown/compare/v0.0.10...v0.0.11) - 2026-10-28

### Other Changes
- refactor: share explicit zero SVG length detection by @Songmu in https://github.com/Songmu/slidown/pull/110
- Refactor template design-part relationship lookup by @Songmu in https://github.com/Songmu/slidown/pull/109
- refactor(md): simplify default condition conversion by @Songmu in https://github.com/Songmu/slidown/pull/108
- Refactor table region style selection by @Songmu in https://github.com/Songmu/slidown/pull/112
- refactor(config): share XDG home path resolution by @Songmu in https://github.com/Songmu/slidown/pull/113
- refactor(render): share placeholder position ordering by @Songmu in https://github.com/Songmu/slidown/pull/114
- refactor(pptx): share fingerprint element serialization by @Songmu in https://github.com/Songmu/slidown/pull/115
- refactor(svgshape): split color parsing by representation by @Songmu in https://github.com/Songmu/slidown/pull/116
- refactor(pptx): separate table cell defaults and properties by @Songmu in https://github.com/Songmu/slidown/pull/117
- refactor(svgshape): isolate XML node construction by @Songmu in https://github.com/Songmu/slidown/pull/118
- Deduplicate slide-order fixtures and SVG dimension tests by @Songmu in https://github.com/Songmu/slidown/pull/119
- test(pptx): protect shape-key stamping XML and uniqueness by @Songmu in https://github.com/Songmu/slidown/pull/120
- refactor(svgshape): separate CSS selector validation by @Songmu in https://github.com/Songmu/slidown/pull/121
- refactor(cmd): isolate the watch apply event loop by @Songmu in https://github.com/Songmu/slidown/pull/122
- refactor(svgshape): clarify SVG path bounds accumulation by @Songmu in https://github.com/Songmu/slidown/pull/123

## [v0.0.10](https://github.com/Songmu/slidown/compare/v0.0.9...v0.0.10) - 2026-10-28

### Other Changes
- Fix fontconfig discovery for LibreOffice visual tests by @Songmu in https://github.com/Songmu/slidown/pull/89
- Refactor SVG path parsing into explicit state and command handlers by @Songmu in https://github.com/Songmu/slidown/pull/92
- Refactor slide alignment into explicit matching phases by @Songmu in https://github.com/Songmu/slidown/pull/93
- Refactor Markdown traversal into explicit walk state by @Songmu in https://github.com/Songmu/slidown/pull/91
- Refactor incremental slide reuse into focused restoration helpers by @Songmu in https://github.com/Songmu/slidown/pull/94
- chore: restore staticcheck compatibility with Go 1.27 by @Songmu in https://github.com/Songmu/slidown/pull/95
- refactor: clarify presentation update stages by @Songmu in https://github.com/Songmu/slidown/pull/96
- Refactor SVG conversion into responsibility-based stages by @Songmu in https://github.com/Songmu/slidown/pull/100
- refactor: clarify shape parsing and incremental reuse by @Songmu in https://github.com/Songmu/slidown/pull/99
- refactor: clarify SVG external resource scanning by @Songmu in https://github.com/Songmu/slidown/pull/97
- test: share ZIP helpers and organize SVG regressions by @Songmu in https://github.com/Songmu/slidown/pull/101
- refactor(md): clarify fragment conversion and default evaluation by @Songmu in https://github.com/Songmu/slidown/pull/98
- Fix list indentation defaults and preserve template text styles by @Songmu in https://github.com/Songmu/slidown/pull/102
- Test the real apply entrypoint instead of duplicating its pipeline by @Songmu in https://github.com/Songmu/slidown/pull/103
- Separate SVG fill and stroke paint eligibility stages by @Songmu in https://github.com/Songmu/slidown/pull/105
- Split template loading into focused stages and simplify preservation tests by @Songmu in https://github.com/Songmu/slidown/pull/106
- Share SVG number scanning and separate transform matrix construction by @Songmu in https://github.com/Songmu/slidown/pull/104
- chore(deps): bump Songmu/tagpr from 1.21.3 to 1.21.4 in the dependencies group by @dependabot[bot] in https://github.com/Songmu/slidown/pull/107

## [v0.0.9](https://github.com/Songmu/slidown/compare/v0.0.8...v0.0.9) - 2026-10-27

### Other Changes
- chore(deps): bump the dependencies group across 1 directory with 6 updates by @dependabot[bot] in https://github.com/Songmu/slidown/pull/82
- chore(deps): bump the dependencies group across 1 directory with 3 updates by @dependabot[bot] in https://github.com/Songmu/slidown/pull/84
- chore(deps): bump the dependencies group with 4 updates by @dependabot[bot] in https://github.com/Songmu/slidown/pull/87
- chore(deps): bump golang.org/x/sync from 0.23.0 to 0.24.0 in the dependencies group by @dependabot[bot] in https://github.com/Songmu/slidown/pull/86
- Fix broken notes relationships when reusing slides by @Songmu in https://github.com/Songmu/slidown/pull/88

## [v0.0.8](https://github.com/Songmu/slidown/compare/v0.0.7...v0.0.8) - 2026-07-15

### Other Changes
- chore(deps): bump the dependencies group with 3 updates by @dependabot[bot] in https://github.com/Songmu/slidown/pull/74
- chore(deps): bump Songmu/tagpr from 1.20.0 to 1.20.1 in the dependencies group by @dependabot[bot] in https://github.com/Songmu/slidown/pull/75
- Convert SVG images to native pptx shapes with image fallback by @Songmu in https://github.com/Songmu/slidown/pull/73

## [v0.0.7](https://github.com/Songmu/slidown/compare/v0.0.6...v0.0.7) - 2026-07-10

### Other Changes
- Order body placeholders by visual position by @Songmu in https://github.com/Songmu/slidown/pull/69
- Make the leading slide the title slide unless it is ignored by @Songmu in https://github.com/Songmu/slidown/pull/71
- Re-render slides when the implicit first-slide layout changes by @Songmu in https://github.com/Songmu/slidown/pull/72

## [v0.0.6](https://github.com/Songmu/slidown/compare/v0.0.5...v0.0.6) - 2026-07-10

### Other Changes
- Render "skip" pages as hidden slides instead of dropping them by @Songmu in https://github.com/Songmu/slidown/pull/68
- chore(deps): bump github.com/google/cel-go from 0.28.1 to 0.29.1 in the dependencies group by @dependabot[bot] in https://github.com/Songmu/slidown/pull/67

## [v0.0.5](https://github.com/Songmu/slidown/compare/v0.0.4...v0.0.5) - 2026-07-06

### Other Changes
- Document that placeholder overflow handling differs from deck and may change by @Songmu in https://github.com/Songmu/slidown/pull/61
- Resolve style-layout theme colors (`schemeClr`/`sysClr`) via theme `clrScheme` + master/layout color mapping by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/63
- Preserve hand-added non-placeholder text boxes via stamped shape keys by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/64
- Add `apply --watch` with fswatcher-based rebuild loop and debounce by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/65

## [v0.0.4](https://github.com/Songmu/slidown/compare/v0.0.3...v0.0.4) - 2026-07-06

### Other Changes
- Isolate TestResolveTemplate from the local config by @Songmu in https://github.com/Songmu/slidown/pull/56
- Add shape-level incremental reuse to preserve manual edits by @Songmu in https://github.com/Songmu/slidown/pull/58
- Read existing .pptx once per apply instead of up to 4 times by @Songmu in https://github.com/Songmu/slidown/pull/59
- Stamp deck keys onto slides to make the source authoritative by @Songmu in https://github.com/Songmu/slidown/pull/60

## [v0.0.3](https://github.com/Songmu/slidown/compare/v0.0.2...v0.0.3) - 2026-07-05

### Other Changes
- Fix dropped title-only edits (preserve unmanaged parts); remove dead code by @Songmu in https://github.com/Songmu/slidown/pull/50
- Reduce boilerplate in pptx package tests by @Songmu in https://github.com/Songmu/slidown/pull/51
- Distribute multiple subtitles across multiple subtitle placeholders by @Songmu in https://github.com/Songmu/slidown/pull/52
- Strengthen visual test coverage; drop brittle subtitle XML golden by @Songmu in https://github.com/Songmu/slidown/pull/53
- Bind images to picture placeholders in template layouts by @Songmu in https://github.com/Songmu/slidown/pull/54
- Support deck-compatible "style" layout for design overrides by @Songmu in https://github.com/Songmu/slidown/pull/55

## [v0.0.2](https://github.com/Songmu/slidown/compare/v0.0.1...v0.0.2) - 2026-07-04

### Other Changes
- Drop stale design parts when merging over an existing deck by @Songmu in https://github.com/Songmu/slidown/pull/43
- Carry over template notes/handout masters and stop orphaning their themes by @Songmu in https://github.com/Songmu/slidown/pull/45
- Merge version package into root package by @Songmu in https://github.com/Songmu/slidown/pull/46
- Warn when a slide's specified layout is not found in the template by @Songmu in https://github.com/Songmu/slidown/pull/47
- Restrict apply --template to new-file creation by @Songmu in https://github.com/Songmu/slidown/pull/48

## [v0.0.1](https://github.com/Songmu/slidown/commits/v0.0.1) - 2026-07-01

### Other Changes
- Rewrite deck into slidown: Markdown → PowerPoint (.pptx) by @Songmu in https://github.com/Songmu/slidown/pull/3
- chore(deps): bump golang.org/x/sync from 0.20.0 to 0.21.0 in the dependencies group across 1 directory by @dependabot[bot] in https://github.com/Songmu/slidown/pull/4
- docs: add design.md (architecture & spec) by @Songmu in https://github.com/Songmu/slidown/pull/5
- Rename build subcommand to apply by @Songmu in https://github.com/Songmu/slidown/pull/7
- Preserve template Default content types so PowerPoint stops asking to repair by @Songmu in https://github.com/Songmu/slidown/pull/8
- Promote subtitle-hinted body placeholders so users can opt in without XML edits by @Songmu in https://github.com/Songmu/slidown/pull/9
- Run visual e2e tests by default on Linux CI with Noto Sans fonts by @Songmu in https://github.com/Songmu/slidown/pull/10
- Replace template_base.pptx fixture with the deck integration template by @Songmu in https://github.com/Songmu/slidown/pull/11
- chore(deps): bump the dependencies group with 2 updates by @dependabot[bot] in https://github.com/Songmu/slidown/pull/12
- Restructure README to position slidown as a deck sibling project by @Songmu in https://github.com/Songmu/slidown/pull/13
- Accept .potx PowerPoint templates alongside .pptx by @Songmu in https://github.com/Songmu/slidown/pull/14
- Fix non-deterministic ZIP part order in MergeReusingUnchangedSlides by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/25
- Create state dir before writing error dump by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/22
- Read slide metadata in `sldIdLst` order to fix keyless reuse on reordered decks by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/26
- Sanitize XML 1.0-forbidden characters in OOXML output by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/24
- Fix hyperlinks in table cells producing dangling r:id references by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/17
- Preserve incremental slide reuse when apply uses a template by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/18
- unexport dead md.ApplyFrontmatterToMD by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/40
- pptx: GC unreferenced media when loading template to prevent monotonic growth by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/39
- Fix slide reuse corruption when sldIdLst order differs from filename order by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/30
- fix: drop sync.OnceValue from XDG path getters; isolate tests from user config by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/36
- Implement multi-body placeholder distribution in template renderer by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/37
- Guard slide reuse against template switches by embedding a design-hash fingerprint by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/34
- fix: prevent media name collisions in MergeReusingUnchangedSlides by @Songmu with @Copilot in https://github.com/Songmu/slidown/pull/32
- render: dedupe subtitle/body helpers and fix title-layout selection with leading Skip slide by @Songmu in https://github.com/Songmu/slidown/pull/41
- chore(deps): bump the dependencies group with 2 updates by @dependabot[bot] in https://github.com/Songmu/slidown/pull/42
