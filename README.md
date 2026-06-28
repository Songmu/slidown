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

### Using a template

Supply a `.pptx` (or `.potx`) whose theme, slide masters and layouts should
be used as the design:

```console
$ slidown apply deck.md --template theme.pptx
```

The template can also be set with the `template` frontmatter or config
field. See [docs/templates.md](docs/templates.md) for layout selection,
inspecting available layouts with `ls-layouts`, and repurposing existing
placeholders as subtitle targets.

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
- `template` (string): Path to a `.pptx` or `.potx` file whose theme, slide
  masters and layouts are used as the design for the generated deck.
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
line-break handling.

## Configuration file

`slidown` reads optional global configuration:

1. `${XDG_CONFIG_HOME:-~/.config}/slidown/config-{profile}.yml` (with `--profile`)
2. `${XDG_CONFIG_HOME:-~/.config}/slidown/config.yml`

```yaml
breaks: true
codeBlockToImageCommand: "go run ./cmd/txt2img"
template: "theme.pptx"
```

Settings in frontmatter take precedence over the configuration file.

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
