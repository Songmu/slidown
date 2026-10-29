package md

import (
	"strings"
	"testing"
)

func TestExpandTemplate(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		store     map[string]any
		expected  string
		errPrefix string
	}{
		{
			name:     "simple variable substitution",
			template: "Hello {{name}}!",
			store:    map[string]any{"name": "World"},
			expected: "Hello World!",
		},
		{
			name:     "multiple variables",
			template: "{{lang}} code: {{content}}",
			store:    map[string]any{"lang": "go", "content": "fmt.Println(\"hello\")"},
			expected: "go code: fmt.Println(\"hello\")",
		},
		{
			name:     "dot notation for nested maps",
			template: "Home directory: {{env.HOME}}",
			store: map[string]any{
				"env": map[string]any{"HOME": "/home/user"},
			},
			expected: "Home directory: /home/user",
		},
		{
			name:     "map[string]string access",
			template: "Path: {{env.PATH}}",
			store: map[string]any{
				"env": map[string]string{"PATH": "/usr/bin"},
			},
			expected: "Path: /usr/bin",
		},
		{
			name:     "integer values",
			template: "Count: {{count}}",
			store:    map[string]any{"count": 42},
			expected: "Count: 42",
		},
		{
			name:     "boolean values",
			template: "Enabled: {{enabled}}",
			store:    map[string]any{"enabled": true},
			expected: "Enabled: true",
		},
		{
			name:     "empty string value",
			template: "Value: {{empty}}",
			store:    map[string]any{"empty": ""},
			expected: "Value: ",
		},
		{
			name:     "no variables to expand",
			template: "No variables here",
			store:    map[string]any{"unused": "value"},
			expected: "No variables here",
		},
		{
			name:     "variable with spaces",
			template: "{{ name }}",
			store:    map[string]any{"name": "value"},
			expected: "value",
		},
		{
			name:     "CEL ternary operator - true condition",
			template: `{{lang == "" ? "md" : lang}}`,
			store:    map[string]any{"lang": ""},
			expected: "md",
		},
		{
			name:     "CEL ternary operator - false condition",
			template: `{{lang == "" ? "md" : lang}}`,
			store:    map[string]any{"lang": "go"},
			expected: "go",
		},
		{
			name:     "CEL arithmetic expression",
			template: "Result: {{count + 10}}",
			store:    map[string]any{"count": 32},
			expected: "Result: 42",
		},
		{
			name:     "CEL string concatenation",
			template: `{{prefix + " " + suffix}}`,
			store:    map[string]any{"prefix": "Hello", "suffix": "World"},
			expected: "Hello World",
		},
		{
			name:     "CEL logical expressions",
			template: "{{enabled && count > 0}}",
			store:    map[string]any{"enabled": true, "count": 5},
			expected: "true",
		},
		{
			name:     "CEL string functions",
			template: "{{name.size()}}",
			store:    map[string]any{"name": "test"},
			expected: "4",
		},
		{
			name:     "real-world code generation example",
			template: `convert {{lang == "" ? "md" : lang}} -o {{output}} < {{content}}`,
			store: map[string]any{
				"lang":    "go",
				"content": "main.go",
				"output":  "/tmp/out.png",
			},
			expected: "convert go -o /tmp/out.png < main.go",
		},
		{
			name:     "environment variables with ternary",
			template: `HOME={{env.HOME}} USER={{env.USER == "" ? "default" : env.USER}}`,
			store: map[string]any{
				"env": map[string]string{
					"HOME": "/home/testuser",
					"USER": "testuser",
				},
			},
			expected: "HOME=/home/testuser USER=testuser",
		},
		{
			name:     "complex nested expression",
			template: `{{config.database.host + ":" + string(config.database.port)}}`,
			store: map[string]any{
				"config": map[string]any{
					"database": map[string]any{
						"host": "localhost",
						"port": 5432,
					},
				},
			},
			expected: "localhost:5432",
		},
		{
			name:      "undefined variable",
			template:  "{{undefined}}",
			store:     map[string]any{"other": "value"},
			errPrefix: "template compilation error for '{{undefined}}':",
		},
		{
			name:      "invalid CEL expression",
			template:  "{{lang == }}",
			store:     map[string]any{"lang": "go"},
			errPrefix: "template compilation error for '{{lang ==}}':",
		},
		{
			name:     "multiple CEL expressions in one template",
			template: `{{lang}} version {{version}} {{enabled ? "enabled" : "disabled"}}`,
			store: map[string]any{
				"lang":    "Go",
				"version": "1.21",
				"enabled": true,
			},
			expected: "Go version 1.21 enabled",
		},
		{
			name:     "floating point arithmetic",
			template: "{{ratio * 2.0}}",
			store:    map[string]any{"ratio": 1.25},
			expected: "2.5",
		},
		{
			name:     "string list indexing",
			template: "{{names[1]}}",
			store:    map[string]any{"names": []string{"first", "second"}},
			expected: "second",
		},
		{
			name:     "mixed list indexing",
			template: "{{values[0]}}:{{values[1]}}",
			store:    map[string]any{"values": []any{"count", 2}},
			expected: "count:2",
		},
		{
			name:     "null value",
			template: "{{value == null}}",
			store:    map[string]any{"value": nil},
			expected: "true",
		},
		{
			name:      "missing map key",
			template:  "prefix {{env.MISSING}} suffix",
			store:     map[string]any{"env": map[string]string{"HOME": "/home/user"}},
			errPrefix: "template evaluation error for '{{env.MISSING}}':",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandTemplate(tt.template, tt.store)

			if tt.errPrefix != "" {
				if err == nil || !strings.HasPrefix(err.Error(), tt.errPrefix) {
					t.Errorf("expandTemplate() error = %v, want prefix %q", err, tt.errPrefix)
				}
				if result != "" {
					t.Errorf("expandTemplate() returned partial result %q on error", result)
				}
				return
			}

			if err != nil {
				t.Errorf("expandTemplate() unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expandTemplate() = %q, want %q", result, tt.expected)
			}
		})
	}
}
