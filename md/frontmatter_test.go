package md

import (
	"testing"

	"github.com/Songmu/slidown/config"
	"github.com/google/go-cmp/cmp"
)

func TestFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		want     *Frontmatter
	}{
		{
			name: "with frontmatter",
			markdown: `---
title: Test Title
author: Test Author
tags:
  - tag1
  - tag2
---

# Slide Title

Content`,
			want: &Frontmatter{},
		},
		{
			name: "without frontmatter",
			markdown: `# Slide Title

Content`,
			want: nil,
		},
		{
			name:     "empty frontmatter",
			markdown: "---\n---\n\n# Slide Title",
			want:     &Frontmatter{},
		},
		{
			name: "frontmatter with trailing delimiter",
			markdown: `---
title: Test
---
# Slide Title`,
			want: &Frontmatter{},
		},
		{
			name: "frontmatter with any fields (all ignored)",
			markdown: `---
title: Test Title
author: Test Author
unknown_field: ignored
custom_data: 
  nested: value
metadata:
  key1: value1
  key2: value2
tags:
  - tag1
  - tag2
---
# Slide Title`,
			want: &Frontmatter{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, err := Parse(".", []byte(tt.markdown), nil)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if md == nil {
				t.Fatal("Parse() returned nil md")
				return
			}

			got := md.Frontmatter

			// Check if frontmatter matches expected value
			if tt.want == nil {
				if got != nil {
					t.Errorf("Parse() frontmatter = %+v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("Parse() frontmatter = nil, want non-nil empty struct")
				return
			}

			// Since Frontmatter is an empty struct, just verify it's not nil when expected
			// All YAML fields are ignored, so no field comparison needed
		})
	}
}

func TestApplyConfig(t *testing.T) {
	tests := []struct {
		name               string
		initialFrontmatter *Frontmatter
		config             *config.Config
		want               *Frontmatter
	}{
		{
			name: "Apply config breaks when frontmatter breaks is not set",
			initialFrontmatter: &Frontmatter{
				Breaks: nil, // not set
			},
			config: &config.Config{
				Breaks: new(true),
			},
			want: &Frontmatter{
				Breaks:   new(true),
				Defaults: nil,
			},
		},
		{
			name: "Keep existing breaks value when already set",
			initialFrontmatter: &Frontmatter{
				Breaks: new(false), // already set
			},
			config: &config.Config{
				Breaks: new(true),
			},
			want: &Frontmatter{
				Breaks:   new(false), // keep existing value
				Defaults: nil,
			},
		},
		{
			name:               "Apply breaks when frontmatter is nil",
			initialFrontmatter: nil,
			config: &config.Config{
				Breaks: new(true),
			},
			want: &Frontmatter{
				Breaks:   new(true),
				Defaults: nil,
			},
		},
		{
			name: "Add config defaults when no existing defaults conditions",
			initialFrontmatter: &Frontmatter{
				Defaults: []DefaultCondition{}, // empty slice
			},
			config: &config.Config{
				Defaults: []config.DefaultCondition{
					{
						If:     "page == 1",
						Layout: "title",
						Freeze: new(true),
					},
				},
			},
			want: &Frontmatter{
				Breaks: nil,
				Defaults: []DefaultCondition{
					{
						If:     "page == 1",
						Layout: "title",
						Freeze: new(true),
					},
				},
			},
		},
		{
			name: "Append config defaults when existing defaults conditions present",
			initialFrontmatter: &Frontmatter{
				Defaults: []DefaultCondition{
					{
						If:     "page == 2",
						Layout: "content",
						Skip:   new(true),
					},
				},
			},
			config: &config.Config{
				Defaults: []config.DefaultCondition{
					{
						If:     "page == 1",
						Layout: "title",
						Freeze: new(true),
					},
					{
						If:     "page == 3",
						Layout: "end",
						Ignore: new(true),
					},
				},
			},
			want: &Frontmatter{
				Breaks: nil,
				Defaults: []DefaultCondition{
					{
						If:     "page == 2",
						Layout: "content",
						Skip:   new(true),
					},
					{
						If:     "page == 1",
						Layout: "title",
						Freeze: new(true),
					},
					{
						If:     "page == 3",
						Layout: "end",
						Ignore: new(true),
					},
				},
			},
		},
		{
			name: "Apply config codeBlockToImageCommand when frontmatter codeBlockToImageCommand is not set",
			initialFrontmatter: &Frontmatter{
				CodeBlockToImageCommand: "", // not set
			},
			config: &config.Config{
				CodeBlockToImageCommand: "go run testdata/txt2img/main.go",
			},
			want: &Frontmatter{
				CodeBlockToImageCommand: "go run testdata/txt2img/main.go",
				Defaults:                nil,
			},
		},
		{
			name: "Keep existing codeBlockToImageCommand value when already set",
			initialFrontmatter: &Frontmatter{
				CodeBlockToImageCommand: "go run testdata/txt2img/main.go", // already set
			},
			config: &config.Config{
				CodeBlockToImageCommand: "go run other/command",
			},
			want: &Frontmatter{
				CodeBlockToImageCommand: "go run testdata/txt2img/main.go", // keep existing value
				Defaults:                nil,
			},
		},
		{
			name:               "Apply codeBlockToImageCommand when frontmatter is nil",
			initialFrontmatter: nil,
			config: &config.Config{
				CodeBlockToImageCommand: "go run testdata/txt2img/main.go",
			},
			want: &Frontmatter{
				CodeBlockToImageCommand: "go run testdata/txt2img/main.go",
				Defaults:                nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.initialFrontmatter.applyConfig(tt.config)

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Frontmatter mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
