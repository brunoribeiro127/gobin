package model

// ModuleOrigin represents the origin of a module containing the version control
// system, the URL, the hash and optionally the reference.
type ModuleOrigin struct {
	VCS  string  `json:"VCS"`
	URL  string  `json:"URL"`
	Hash string  `json:"Hash"`
	Ref  *string `json:"Ref"`
}
