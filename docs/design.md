# slidown design & specification

This document describes how `slidown` is structured and how it behaves. It
complements the user-facing [`README.md`](../README.md) and the Markdown
reference in [`docs/markdown.md`](markdown.md).

`slidown` is a fork of [`k1LoW/deck`](https://github.com/k1LoW/deck) rewritten to
generate **PowerPoint (`.pptx`)** from Markdown. It reuses deck's Markdown parser
and content model, but replaces the Google Slides API backend with a pure-Go
Open XML (OOXML) writer. There is no Google account, OAuth, Drive, Chrome or
network dependency.

## Goals

- Markdown for content, PowerPoint for delivery; the `.pptx` is the deliverable.
- Keep deck's Markdown format and element-to-placeholder mapping as close as
  possible.
- Pure Go, MIT licensed: build the OOXML package directly with no third-party
  Office libraries.
- Support incremental rebuilds that preserve manual edits to unchanged slides.

## Non-goals / out of scope

- **Shape-level (intra-slide) diffing.** Reuse granularity is a whole slide; a
  slide is either kept verbatim or fully regenerated (see
  [Incremental rebuild](#incremental-rebuild)).
- Anything that required Google: Slides/Drive APIs, OAuth, the `ls`/`open`/`new`/
  `doctor`/`export` commands, Drive-based PDF export, watch mode.
- These are documented as possible future work, not current behaviour.

## Architecture

slidown keeps deck's layered structure:

```
Markdown
  │  md package          parse Markdown -> Contents -> slidown.Slides (the model)
  ▼
slidown.Slides           internal slide model (root package)
  │  render package       model -> pptx.Presentation (placeholder/geometry mapping)
  ▼
pptx.Presentation        in-memory OOXML model
  │  pptx package         WriteTo -> .pptx (zip of XML parts)
  ▼
.pptx file
```

### Packages

| Package | Responsibility |
| --- | --- |
| `slidown` (root) | Internal model (`Slide`, `Body`, `Paragraph`, `Fragment`, `Image`, `Table`, `BlockQuote`), image loading, per-slide source fingerprints. |
| `md` | Markdown parsing (goldmark) → `Contents` → `slidown.Slides`; frontmatter, page config, CEL defaults, code-block-to-image. |
| `render` | Maps the slide model onto `pptx` placeholders/shapes and computes geometry. |
| `pptx` | Pure-Go OOXML writer/reader: builds the `.pptx` zip, loads external templates, merges/reuses slides on incremental rebuild. |
| `config` | Global config loading and XDG paths. |
| `cmd` | Cobra CLI (`apply`, `ls-layouts`). |
| `version` | Build/version metadata. |

### Generated OOXML package

Built-in (non-template) mode emits, among others:

- `[Content_Types].xml`, `_rels/.rels`
- `docProps/core.xml` (carries the presentation title), `docProps/app.xml`
- `ppt/presentation.xml` (+ rels), `ppt/presProps.xml`, `ppt/viewProps.xml`
- `ppt/theme/theme1.xml`
- `ppt/slideMasters/slideMaster1.xml` (+ rels)
- `ppt/slideLayouts/slideLayout1.xml` (+ rels) — a single "Title and Content" layout
- `ppt/slides/slideN.xml` (+ rels) per slide
- `ppt/notesMasters/notesMaster1.xml` and `ppt/notesSlides/notesSlideN.xml` when any slide has a speaker note
- `ppt/media/*` for embedded images

All user-controlled text (titles, body runs, table cells, notes, image/shape
names, hyperlink targets, colors) is XML-escaped.

### Template mode

When a `.pptx` template is supplied (`--template` or the `template` config
field when creating a new file, or an existing output file reused as its own
template), `pptx.LoadTemplate`
extracts the template's **design parts** — theme, slide master(s), slide layouts
(with their placeholders) and the `ppt/media` tree — and copies them verbatim.
Generated slides reference the template's layouts; newly generated media is
numbered after the template's highest existing media index to avoid collisions.

`slidown ls-layouts TEMPLATE` prints the available layout names (resolved from a
`.pptx` directly, or from a deck file's configured template).

## Markdown → slide element mapping

Mirrors deck (independent of the concrete layout):

- The **shallowest heading** level on a slide → **title** placeholder
  (`title`/`ctrTitle`). Usually H1 (`#`).
- The **next** heading level → **subtitle** placeholder(s) (`subTitle`). A slide
  may carry more than one subtitle heading at this level.
- Everything else → **body** placeholder(s) (`body`), split into one or more
  bodies by the title/subtitle headings.
- **Images** (`![](...)`) → `p:pic`, laid out within the content area preserving
  aspect ratio. Code blocks can be turned into images via
  `codeBlockToImageCommand`.
- **Tables** (GFM) → `p:graphicFrame` containing `a:tbl`, with a styled header
  row, borders and per-column alignment.
- **Block quotes** → italic, indented body paragraphs.
- **Speaker notes** (HTML comments) → the slide's notes slide.

Subtitle placeholder resolution treats both real `<p:ph type="subTitle"/>`
placeholders and **subtitle-hinted** body placeholders as subtitle slots. A hint
is a body-shaped placeholder whose `<p:cNvPr name="...">` (editable from
PowerPoint's Selection Pane) or custom prompt text (`<a:t>` in `<p:txBody>` of
the layout shape) contains "subtitle" (case-insensitive). This lets users opt in
via GUI without XML editing. Hint-promoted shapes keep their underlying `body`
placeholder type in the emitted slide and carry a slidown role extension
(`<p:extLst>` under `<p:nvPr>` with URI
`{A3F7C812-9B4D-4E16-83CA-2D7F1E9B4C58}`, attribute `role="subTitle"`) so
future incremental shape updates can identify the subtitle target by intent.

When a layout exposes multiple subtitle slots, the slide's subtitles are
distributed across them **one per slot, in visual order** (top to bottom, then
left to right), with the last slot absorbing any overflow. A slot's position is
taken from the layout shape's `<a:xfrm>`, or inherited from the slide master's
placeholder with the same `idx` when the layout omits its own geometry (the
standard OOXML placeholder inheritance). If the geometry of any slot cannot be
resolved, the layout's shape-tree order is used unchanged. When a layout has no
subtitle slot at all, subtitles are folded into the first body placeholder as
emphasized (bold) lead paragraphs so no content is lost.

Inline styles map to run properties: bold, italic, monospace (code), underline,
strikethrough (`del`), and hyperlinks.

Shapes slidown emits within a slide's `p:spTree`: text shapes (`p:sp`, used for
title/subtitle/body placeholders), pictures (`p:pic`) and tables
(`p:graphicFrame`).

## Configuration & inputs

### Frontmatter (YAML, per document)

| Field | Meaning |
| --- | --- |
| `title` | Presentation title; written to `docProps/core.xml`. |
| `output` | Output `.pptx` path (used when `--output` is absent). |
| `breaks` | Render single line breaks as line breaks (default: as spaces). |
| `codeBlockToImageCommand` | Command used to convert code blocks to images. |
| `defaults` | CEL-based conditional page configuration. |

### Global config

`${XDG_CONFIG_HOME:-~/.config}/slidown/config[-{profile}].yml` supports
`breaks`, `defaults`, `codeBlockToImageCommand`, `template`. Precedence:
command-line > frontmatter > config > built-in defaults. The `template`
field is config-only (not a frontmatter field) and seeds only newly created
output files.

### Page configuration

An HTML comment containing JSON configures a single page, e.g.
`<!-- {"layout":"section","key":"intro","freeze":true} -->`:

| Field | Meaning |
| --- | --- |
| `layout` | Template layout name to use for the page. |
| `key` | Stable, opaque per-page identity used for incremental matching. |
| `freeze` | Pin the page: keep its existing slide on rebuild even if the source changed. |
| `skip` | Generate the slide but mark it skipped (hidden). |
| `ignore` | Exclude the page from generation entirely. |

`defaults` apply CEL expressions (e.g. `page == 1`, `titles.size() == 1`) to set
page configuration for pages that don't configure it explicitly.

### Code blocks to images

`codeBlockToImageCommand` runs an external command per code block. The command
receives the language and content via environment variables (`CODEBLOCK_LANG`,
`CODEBLOCK_CONTENT`, `CODEBLOCK_OUTPUT`) and via CEL template placeholders
(`{{lang}}`, `{{content}}`, `{{output}}`, `{{env.X}}`). Image data is read from
stdout, or from the `{{output}}` file when used.

## Incremental rebuild

When the output `.pptx` already exists, `apply` rebuilds **in place**, reusing
the existing slide parts for slides whose source has not meaningfully changed (or
that are frozen). This preserves manual edits made in PowerPoint to those slides.
The decision is per slide; there is no shape-level diff.

### Change detection via embedded source fingerprints

The earlier approach reverse-parsed the generated `.pptx` and compared it to the
markdown-derived model. That reverse parse was lossy (it could not recover, for
example, rich title formatting or run underline), so genuine changes could be
read as "unchanged" and silently dropped. slidown instead embeds a **fingerprint
of the source slide** into each slide and compares source-to-source.

- At generation time each slide carries a fingerprint (and its stable `key`) in
  the slide's `extLst` (a slidown-specific extension; invisible to the
  presentation and preserved verbatim when a slide is reused). If another tool
  strips the extension, the slide simply looks "changed" and is regenerated — a
  safe degradation, never silent loss of a real change.
- The fingerprint is a JSON signature with two parts:
  - **Non-image content** (layout, titles, subtitles, bodies, block quotes,
    tables, speaker note, freeze/skip) is hashed (SHA-256) and compared
    **exactly**. Any change, including inline formatting such as bold/underline/
    strikethrough, forces a regenerate.
  - **Images** are recorded by their **perceptual hash** (with a content checksum
    fallback) plus link, and compared **order-independently** within a small
    Hamming-distance threshold. Recompressing, reordering or repositioning an
    image therefore counts as unchanged. Because the comparison is
    source-to-source, an image that PowerPoint re-compresses inside the `.pptx`
    never affects the decision.

### Identity matching by key

Slides are matched to their existing counterpart by **stable `key`** when both
sides have one, falling back to **positional** matching for keyless slides. A
matched slide is reused when it is frozen or when its source fingerprint still
matches. Because matching is key-based, reuse and `freeze` follow a slide across
inserts, deletions and reordering rather than being tied to its position.

A key that no longer appears in the deck source is **orphaned** (its page was
renamed or removed). An orphaned-key slide may be re-paired positionally only by
a **keyed** source slide — a likely key rename — so a renamed page can re-claim
its slide, while a keyless page can't divert onto an unrelated removed keyed
slide. A key still present stays reserved so a keyless page cannot steal a merely
reordered slide.

### Authoritative key stamping

The deck source is authoritative for keys. As the final step of every `apply`,
`pptx.StampSlideKeys` rewrites each output slide's `key` (the `k` attribute of
the slidown extension) to match the key of the deck page occupying that visible
position: renamed keys are updated, removed keys are cleared (so no orphaned keys
accumulate), and a slide that carries no slidown metadata — such as one **pasted
in from another presentation** — is tagged with its page's key. Only the `extLst`
is touched, which the fingerprint does not cover, so stamping never perturbs
change detection and is idempotent.

This makes importing a slide self-healing: declare a keyed, frozen placeholder
page for it in the Markdown, paste the slide at that position in PowerPoint, and
the first rebuild pairs them by position, keeps the pasted slide via `freeze`,
and stamps the key onto it; subsequent rebuilds then match it by key regardless
of reordering. `freeze: true` lives only in the Markdown and is never written to
the slide.

### Applying the reuse

`writePresentation` (via `buildReuseMap`) computes a map of *new position →
existing position* for the reusable slides and calls
`pptx.MergeReusingUnchangedSlides`, which starts from the freshly generated
package and, for each reused slide:

- copies the existing slide XML (and rels) to the new position;
- copies the media it references;
- copies the notes slide it references, rewriting the slide ↔ notes-slide cross
  references when the position changes, restoring the notes master if needed, and
  injecting the notes content-type overrides so the package stays valid.

If every slide is reused at its original position and the count is unchanged, the
existing file is left untouched (a no-op). When no slides can be reused (e.g. the
whole deck changed), the rebuild falls back to `pptx.MergeWithExisting`, a plain
ZIP overlay that preserves non-regenerated parts (such as `docProps`) while
dropping stale regenerated parts. An existing output is always rebuilt using
itself as the template — `apply` rejects `--template` when the output already
exists — so a reused slide's layout relationships remain valid.

## CLI

- `slidown apply DECK_FILE [-o OUT.pptx] [--template T.pptx] [--code-block-to-image-command CMD]`
  — generate (or rebuild) a `.pptx`.
- `slidown ls-layouts TEMPLATE` — list a template's layout names.

## Testing

- **Unit tests** across `pptx`, `render`, `md` and `cmd`, including regression
  tests for the OOXML corruption bugs found in review and for fingerprint/key
  based reuse.
- **Golden JSON tests** for the Markdown parser (`md`).
- **External-template fixture** (`testdata/template_base.pptx`, authored by
  LibreOffice) exercising the template loader against foreign OOXML.
- **Visual golden test** (`render/visual_test.go`): renders slides to PNG via
  LibreOffice + `pdftoppm` and compares against committed goldens with
  perceptual hashing. Runs by default; skipped under `go test -short` or when
  the required tools are missing.
- Lint is `go vet` + `staticcheck`, wired through the Go `tool` directive
  (`make lint`), so no extra binary install is required.

## Known limitations / future work

- **Shape-level incremental apply.** Updating only the changed shapes within a
  slide (leaving manually edited shapes untouched) would require stable per-shape
  identity and surgical OOXML patching. deck achieved the equivalent through the
  Google Slides object model + batchUpdate; slidown only reuses or regenerates a
  slide as a whole.
- A `codeBlockToImageCommand` taken from a document's own frontmatter is executed
  during `apply`; processing untrusted Markdown that sets it can run arbitrary
  commands. This mirrors deck and is currently left as-is.
