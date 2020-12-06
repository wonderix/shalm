package shalm

import (
	"github.com/spf13/pflag"
)

// DeleteOptions -
type DeleteOptions struct {
	force     bool
	recursive bool
}

// AddFlags -
func (s *DeleteOptions) AddFlags(flagsSet *pflag.FlagSet) {
	flagsSet.BoolVar(&s.force, "force", false, "Force deletion of charts, even if they are used by others")
	flagsSet.BoolVar(&s.recursive, "recursive", false, "Recursive delete dependencies")
}
