# slidown

`slidown` is a tool for creating PowerPoint (`.pptx`) presentations from Markdown.

It is a sibling project of [`k1LoW/deck`](https://github.com/k1LoW/deck), a
Markdown → Google Slides tool by [@k1LoW](https://github.com/k1LoW). `slidown`
shares `deck`'s philosophy of *Markdown for content, slide tooling for design*,
and adopts the same Markdown format and element mapping. The difference is the
output target: `slidown` writes standalone `.pptx` files via a pure-Go OOXML
writer, with no third-party Office dependencies.

`deck` and `slidown` are designed to be used together or interchangeably from
the same Markdown source, so you can pick the right delivery target — Google
Slides or PowerPoint — without rewriting your slides.

## Installation

**Homebrew:**

```console
$ brew install Songmu/tap/slidown
```

**go install:**

```console
$ go install github.com/Songmu/slidown/cmd/slidown@latest
```

## Usage

### Quick start

Write your slides in Markdown:

```markdown
---
title: Talk about slidown
---

# First slide

## A subtitle

- a bullet point
- **bold** and *italic* and `code`
  - a nested point

---

# Second slide

A paragraph with a [link](https://example.com).
```

Then apply it to a `.pptx`:

```console
$ slidown apply deck.md
Wrote deck.pptx (2 slide(s))
```

By default the output file is the input file name with a `.pptx` extension.
Override it with `--output`/`-o`, or with the `output` frontmatter field:

```console
$ slidown apply deck.md -o talk.pptx
```

### Incremental updates

If the output `.pptx` already exists, `apply` updates it in place. Slides
whose source content has not changed keep their existing slide parts
verbatim, so manual edits made in PowerPoint to unchanged slides are
preserved; only changed slides are regenerated:

```console
$ slidown apply deck.md -o deck.pptx
Updated deck.pptx (2 slide(s))
```

Image-only differences (recompression, reordering or repositioning) are
treated as unchanged, so adjusting images in PowerPoint does not trigger a
regenerate.

To make reuse robust against page reordering, give a page a stable `key` —
otherwise pages are matched by position:

```markdown
# Overview

<!-- {"key": "overview"} -->
```

A page can also be **frozen** with the `freeze` page configuration. A
frozen page keeps its existing slide as-is on rebuild even if its Markdown
changed — useful for pinning a slide you have hand-tuned in PowerPoint:

```markdown
# Hand-tuned slide

<!-- {"freeze": true} -->
```

To bring in a slide **pasted from another presentation**, declare a keyed,
frozen placeholder page for it in the Markdown and paste the slide at that
position in PowerPoint:

```markdown
# Imported slide

<!-- {"key": "imported-architecture", "freeze": true} -->
```

On rebuild, `apply` pairs the placeholder with the pasted slide by position,
keeps the pasted slide verbatim (`freeze`), and stamps the `key` onto it — so
later rebuilds match it by key even after reordering. The deck source is
authoritative for keys: a key renamed or removed in the Markdown is updated or
cleared on the slide accordingly, and `freeze: true` need only stay in the
Markdown.

### Watch mode

Use `--watch` (or `-w`) to keep `apply` running and automatically re-apply
when the deck markdown file changes:

```console
$ slidown apply deck.md --watch
Wrote deck.pptx (2 slide(s))
Updated deck.pptx (2 slide(s))
```

`slidown` watches the deck file's directory so editor atomic-save patterns
(`rename`/`remove` + `create`) are handled, filters events to the deck file,
and debounces bursts so one save triggers one rebuild. If a rebuild fails, the
error is printed and watch mode keeps running until you stop it with `Ctrl-C`.

`--watch` and `--template` are mutually exclusive. `--template` is only for
initial generation; when you need watch mode with a template, set `template`
in the config file instead.

MVP scope: watch mode currently tracks deck markdown file changes only.

### Using a template

Supply a `.pptx` (or `.potx`) whose theme, slide masters and layouts should
be used as the design when creating a new output file:

```console
$ slidown apply deck.md --template theme.pptx
```

A template can only be supplied when the output file does not yet exist. It
may also be set with the `template` config field. When the output already
exists it is updated in place, reusing itself as the template, and passing
`--template` is an error (choose a different `--output`, or remove the file
first). See [docs/templates.md](docs/templates.md) for layout selection,
inspecting available layouts with `ls-layouts`, and repurposing existing
placeholders as subtitle targets. Templates can also include a
[`style` layout](docs/templates.md#style-layout) to customize inline syntax and
table styling. Incremental updates reuse unchanged slides from the existing
output file and still honor `freeze: true`.

## Markdown file format

The Markdown used by `slidown` consists of an optional YAML frontmatter and
a body. Slides are separated by a line of three or more hyphens (`---`).

```markdown
---
title: Talk about slidown
output: talk.pptx
---

# First Slide

Content...
```

### Frontmatter fields

- `title` (string): The title of the presentation, written to the generated
  `.pptx` document properties (metadata).
- `output` (string): Output `.pptx` path (used when `--output` is not given).
- `breaks` (boolean): Control how single line breaks are rendered. Default
  (`false`) renders them as spaces; `true` preserves them as line breaks.
- `codeBlockToImageCommand` (string): Command used to convert code blocks
  to images (see [Code blocks to images](#code-blocks-to-images)).
- `defaults` (array): Conditional page configuration using CEL expressions.

### Markdown specification

`slidown` follows the same Markdown specification as `deck`: CommonMark
plus selected GitHub Flavored Markdown extensions (tables, strikethrough),
with a restricted set of raw inline HTML for text-level semantics
(`<mark>`, `<kbd>`, `<sub>`, `<sup>`, `<u>`, …). Speaker notes are written
as HTML comments (`<!-- ... -->`).

Within each slide, headings are mapped to placeholders by depth:

- The **shallowest** heading on the slide → **title**
- The **next** heading level → **subtitle**
- Everything else → **body**

See [docs/markdown.md](docs/markdown.md) for the full Markdown reference,
including supported/unsupported features, inline-style mappings and
line-break handling. A few edge behaviors (notably how content that overflows
the available placeholders is handled) currently differ from `deck` and are not
yet a stable contract; these are called out inline in that reference.

SVG images are, where possible, converted into native editable PowerPoint
shapes; SVGs using features that cannot be faithfully converted fall back to a
native SVG picture with a rasterized PNG fallback. SVGs that reference external
or relative resources (which can't be packaged) instead get a best-effort
PNG-only rendering. See [Images](docs/markdown.md#images) for details.

## Configuration file

`slidown` reads optional global configuration:

1. `${XDG_CONFIG_HOME:-~/.config}/slidown/config-{profile}.yml` (with `--profile`)
2. `${XDG_CONFIG_HOME:-~/.config}/slidown/config.yml`

```yaml
breaks: true
codeBlockToImageCommand: "go run ./cmd/txt2img"
template: "theme.pptx"
```

Settings in frontmatter take precedence over the configuration file. The
`template` field is honored only from the configuration file (or the
`--template` flag) and only when creating a new output file; it cannot be
set in a deck's frontmatter.

## Code blocks to images

You can convert Markdown code blocks to images by specifying a command that
outputs image data (PNG/JPEG/GIF) to standard output, or to a file via the
`{{output}}` placeholder:

```console
$ slidown apply deck.md --code-block-to-image-command "some-command"
```

This is useful for rendering diagrams (e.g. Mermaid) or syntax-highlighted code
as images.

## Status

`slidown` is under active development. `apply` already reuses unchanged
whole slides; finer-grained intra-slide (sub-element) diffing is future
work. See [docs/design.md](docs/design.md) for the architecture and the
incremental-rebuild design.

## Acknowledgements

- [`k1LoW/deck`](https://github.com/k1LoW/deck) — `slidown` is a sibling
  project of `deck` and reuses its Markdown parsing and content model. The
  Markdown specification, the element-to-slide mapping rules and much of
  the surrounding design come from `deck`. Many thanks to
  [@k1LoW](https://github.com/k1LoW) and the `deck` contributors.

## License

[MIT](LICENSE)
