# Changelog

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
