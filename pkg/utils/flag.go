package utils

import (
	"strings"

	"github.com/spf13/pflag"
)

// WordSepNormalizeFunc changes all flags with separators from "_"  to "-"
func WordSepNormalizeFunc(f *pflag.FlagSet, name string) pflag.NormalizedName {
	if strings.Contains(name, "_") {
		return pflag.NormalizedName(strings.Replace(name, "_", "-", -1))
	}
	return pflag.NormalizedName(name)
}
