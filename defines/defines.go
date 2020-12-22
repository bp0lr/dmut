package defines

//DmutJob desc
type DmutJob struct {
	Domain		string
	Tld        	string
	Sld  		string
	Trd  		string
	Tasks		[]string
}

//Stats desc
type Stats struct{
	Domains int
	Mutations int
	Founds	int
	FoundDomains []string
	WorksToDo []string
}

//LoadStats desc
type LoadStats struct{
	Domains int
	Valid int
	Errors int
}
