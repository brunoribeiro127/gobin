package model

// BinaryDiagnostic represents the diagnostic results for a binary.
type BinaryDiagnostic struct {
	Name                  string
	NotInPath             bool
	DuplicatesInPath      []string
	IsNotManaged          bool
	IsPseudoVersion       bool
	NotBuiltWithGoModules bool
	IsOrphaned            bool
	GoVersion             struct {
		Actual   string
		Expected string
	}
	Platform struct {
		Actual   string
		Expected string
	}
	Retracted       string
	Deprecated      string
	Vulnerabilities []Vulnerability
}

// HasIssues returns whether the binary has any issues.
func (d BinaryDiagnostic) HasIssues() bool {
	return d.NotInPath ||
		len(d.DuplicatesInPath) > 0 ||
		d.IsNotManaged ||
		d.IsPseudoVersion ||
		d.NotBuiltWithGoModules ||
		d.IsOrphaned ||
		d.GoVersion.Actual != d.GoVersion.Expected ||
		d.Platform.Actual != d.Platform.Expected ||
		d.Retracted != "" ||
		d.Deprecated != "" ||
		len(d.Vulnerabilities) > 0
}
