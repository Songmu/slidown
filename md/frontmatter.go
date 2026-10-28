package md

import (
	"reflect"

	"github.com/Songmu/slidown/config"
)

func (fm *Frontmatter) applyConfig(cfg *config.Config) *Frontmatter {
	if cfg == nil || reflect.DeepEqual(*cfg, config.Config{}) {
		return fm
	}
	if fm == nil {
		fm = &Frontmatter{}
	}
	if fm.Breaks == nil {
		fm.Breaks = cfg.Breaks
	}
	if fm.CodeBlockToImageCommand == "" {
		fm.CodeBlockToImageCommand = cfg.CodeBlockToImageCommand
	}
	// append default conditions from config
	for _, cond := range cfg.Defaults {
		fm.Defaults = append(fm.Defaults, DefaultCondition(cond))
	}
	return fm
}
