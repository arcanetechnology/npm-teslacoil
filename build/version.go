package build

// commit is set by the linker, at build time
var commit string

// Version returns the current tlc version
func Version() string {
	return commit
}
