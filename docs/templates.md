# Templates

`slidown` can reuse the theme, slide masters and layouts of an existing
`.pptx` (or `.potx` PowerPoint template) as the design for the generated
deck. Supply the template via the `--template` flag, the `template`
frontmatter field, or the `template` key in the
[configuration file](../README.md#configuration-file).

```console
$ slidown apply deck.md --template theme.pptx
$ slidown apply deck.md --template theme.potx
```

The argument may also be a Markdown deck, in which case its configured
template is resolved.

## Inspecting available layouts

To see which layout names a template provides — useful when selecting a
layout per page via page configuration — use `ls-layouts`:

```console
$ slidown ls-layouts theme.pptx
Title Slide
Title and Content
Section Header
```

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
`Add subtitle here` all qualify. The first body-shaped placeholder that
matches is used as the subtitle slot; the rest remain as body placeholders.
