// Package defines contains the inputs to the legacy mutation rules.
package defines

// DmutJob is a domain split into public suffix, registrable label and subdomain.
type DmutJob struct {
	Domain string
	Tld    string
	Sld    string
	Trd    string
}

// PermutationList contains disable switches. True disables the named rule.
type PermutationList struct {
	AddToDomain  bool
	AddNumbers   bool
	AddSeparator bool
}
