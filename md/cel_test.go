package md

import (
	"strings"
	"testing"
)

func TestReflectDefaultsEvaluationTiming(t *testing.T) {
	tests := []struct {
		name        string
		conditions  []DefaultCondition
		wantErr     string
		wantHeading bool
		wantLayout  string
	}{
		{
			name: "no conditions leave headings untouched",
		},
		{
			name:       "compile error leaves headings untouched",
			conditions: []DefaultCondition{{If: "page =="}},
			wantErr:    "failed to compile expression:",
		},
		{
			name:        "evaluation error follows heading initialization",
			conditions:  []DefaultCondition{{If: "1 / (page - 1) > 0"}},
			wantErr:     "failed to evaluate values:",
			wantHeading: true,
		},
		{
			name:        "false condition initializes headings before later compile error",
			conditions:  []DefaultCondition{{If: "false"}, {If: "page =="}},
			wantErr:     "failed to compile expression:",
			wantHeading: true,
		},
		{
			name: "first match skips later invalid condition",
			conditions: []DefaultCondition{
				{If: "page == 1 && pageTotal == 1 && topHeadingLevel == 3 && headings[1].size() == 0", Layout: "matched"},
				{If: "page =="},
			},
			wantHeading: true,
			wantLayout:  "matched",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := &Content{Headings: map[int][]string{3: {"Heading"}}}
			md := &MD{
				Frontmatter: &Frontmatter{Defaults: tt.conditions},
				Contents:    Contents{content},
			}
			err := md.reflectDefaults()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.HasPrefix(err.Error(), tt.wantErr) {
				t.Fatalf("reflectDefaults() error = %v, want prefix %q", err, tt.wantErr)
			}
			if content.Layout != tt.wantLayout {
				t.Errorf("Layout = %q, want %q", content.Layout, tt.wantLayout)
			}
			for level := 1; level < sentinelLevel; level++ {
				if level == 3 {
					continue
				}
				headings, exists := content.Headings[level]
				if exists != tt.wantHeading || (exists && (headings == nil || len(headings) != 0)) {
					t.Errorf("Headings[%d] = %#v (exists %v), want initialized %v", level, headings, exists, tt.wantHeading)
				}
			}
		})
	}
}

func TestReflectDefaultsNoPagesDoesNotCompile(t *testing.T) {
	md := &MD{Frontmatter: &Frontmatter{Defaults: []DefaultCondition{{If: "page =="}}}}
	if err := md.reflectDefaults(); err != nil {
		t.Fatal(err)
	}
}

func TestReflectDefaultsPreservesExplicitValues(t *testing.T) {
	explicitFreeze, defaultIgnore, defaultSkip := new(false), new(true), new(false)
	content := &Content{
		Layout:   "explicit",
		Freeze:   explicitFreeze,
		Headings: map[int][]string{},
	}
	md := &MD{
		Frontmatter: &Frontmatter{Defaults: []DefaultCondition{
			{If: "true", Layout: "default", Freeze: new(true), Ignore: defaultIgnore, Skip: defaultSkip},
			{If: "true", Layout: "later", Ignore: new(false), Skip: new(true)},
		}},
		Contents: Contents{content},
	}
	if err := md.reflectDefaults(); err != nil {
		t.Fatal(err)
	}
	if content.Layout != "explicit" || content.Freeze != explicitFreeze ||
		content.Ignore != defaultIgnore || content.Skip != defaultSkip {
		t.Errorf("defaults did not preserve explicit values and first-match pointers: %+v", content)
	}
}
