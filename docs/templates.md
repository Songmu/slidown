# Templates

`slidown` can reuse the theme, slide masters and layouts of an existing
`.pptx` (or `.potx` PowerPoint template) as the design for the generated
deck. A template can only be supplied when creating a new output file, via
the `--template` flag or the `template` key in the
[configuration file](../README.md#configuration-file).

```console
$ slidown apply deck.md --template theme.pptx
$ slidown apply deck.md --template theme.potx
```

When the output file already exists it is updated in place, reusing itself
as the template; passing `--template` for an existing output is an error
(choose a different `--output`, or remove the file first).

The `ls-layouts` argument may also be a Markdown deck, in which case the
template is resolved from the `--template` flag or the configuration file.

## Inspecting available layouts

To see which layout names a template provides — useful when selecting a
layout per page via page configuration — use `ls-layouts`:

```console
$ slidown ls-layouts theme.pptx
Title Slide
Title and Content
Section Header
```

## Style layout

Templates can include a special slide layout named exactly `style` to define
inline syntax and table styles. This mirrors
[deck's `style` feature](https://github.com/k1LoW/deck).

The `style` layout is only used as design metadata. It is not selectable as a
content layout, and `slidown ls-layouts` does not list it.

### Inline syntax styles

Add text boxes to the `style` layout. The trimmed text in each text box is the
style keyword, and the text run's formatting becomes the style for that
keyword.

Supported keywords include:

- Markdown syntax names: `bold`, `italic`, `link`, `code`, `del`,
  `blockquote`
- Raw inline HTML element names such as `cite`, `q`, `s`, `kbd`, `samp`,
  `sup`, `sub`, `var`, `em`, `strong`, `u`, and `mark`
- Arbitrary class names used by `<span class="...">`

The following run properties are honored: bold, italic, underline,
strikethrough, text color, highlight/background color, font family, and
superscript/subscript baseline.

Custom styles override slidown's built-in defaults for the same keyword. Syntax
without a custom keyword keeps the built-in style.

### Table styles

Add one 2x2 table to the `style` layout to style generated Markdown tables.
The cells map to table regions as follows:

| Cell | Applies to |
| --- | --- |
| Row 1, column 1 | Header row, first column |
| Row 1, column 2 | Header row, other columns |
| Row 2, column 1 | Data rows, first column |
| Row 2, column 2 | Data rows, other columns |

For each cell, slidown reads the background fill, text style, and horizontal
and vertical alignment. Borders use deck-compatible mapping: the top and left
borders of cell `[0,0]` define the table's outer borders, while each region's
right and bottom borders define inner borders for that region.

## Default layout selection

When no per-page layout is specified, the first slide uses a title-style
layout and the rest use a content-style layout. A specific layout name can
be selected per page via page configuration (an HTML comment with a JSON
object):

```markdown
# Section break

<!-- {"layout": "Section Header"} -->
```

## Repurposing a placeholder as a subtitle target

A layout that has no native `subTitle` placeholder can opt one of its text
placeholders into the subtitle role without XML editing. In PowerPoint's
slide master view, do **either** of the following on the placeholder you
want to use as the subtitle:

- **Rename it** so its name contains "subtitle". On macOS, open
  Home → Arrange → Selection Pane; on Windows, press Alt+F10.
- **Edit its prompt text** to include "subtitle" (for example
  `Add subtitle here`).

`slidown` matches case-insensitively, so labels such as `Subtitle 1` or
`Add subtitle here` all qualify. Every body-shaped placeholder that matches
becomes a subtitle slot; the rest remain as body placeholders.

### Multiple subtitle placeholders

A layout may expose several subtitle-capable placeholders at once — any mix of
native `subTitle` placeholders and hint-named body placeholders. When a slide
has multiple subtitle-level headings, `slidown` distributes them across these
placeholders **one per slot, in visual order** (top to bottom, then left to
right), with the last slot absorbing any overflow. Placeholder position is read
from the layout, or inherited from the slide master when the layout does not
position the placeholder itself. See
[Subtitle Distribution](markdown.md#subtitle-distribution) for the full rules.
