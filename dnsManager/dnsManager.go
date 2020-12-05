package dnsmanager

import (	
	"os"
	"fmt"
	"time"
	"strings"
	"math/rand"
)

//DNSServerEntry desc
type DNSServerEntry struct {
	Host	string
	Status 	bool
	Errors	int
}

var dnsServersTable []DNSServerEntry

//GenerateDNSServersTable desc
func GenerateDNSServersTable(hosts []string)bool{
	
	hosts=validateDNSServers(hosts)

	if(len(hosts) < 1){
		fmt.Printf("There is a problem with the dns server list. Please check manually\n")
		os.Exit(0)
	}

	for _, value:=range hosts{
		d:=DNSServerEntry{value, true, 0}
		dnsServersTable = append(dnsServersTable, d)
	}
	return true
}

//ReturnFullServersList desc
func ReturnFullServersList()([]DNSServerEntry, int){
	return dnsServersTable, len(dnsServersTable)
}

//ReturnRandomDNSServerEntry desc
func ReturnRandomDNSServerEntry() DNSServerEntry{
	
	if(len(dnsServersTable) == 0){
		fmt.Printf("Every DNS Server in our list has over past our error limit. Please try increasing this number using the flag ---dns-errorLimit and playing a bit with --dns-timeout")
		os.Exit(0)
	}

	var pick DNSServerEntry

	for i := 0; i < 100; i++ {
		rand.Seed(time.Now().UnixNano())
		randomIndex := rand.Intn(len(dnsServersTable))

		pick = dnsServersTable[randomIndex]
		if(dnsServersTable[randomIndex].Status == true){
			return pick	
		}		
	}

	fmt.Printf("All our dns server has reported to many errors. I can't continue.")
	os.Exit(0)

	return pick
}

//ReportDNSError desc
func ReportDNSError(host string, errorsLimit int){		

	for id, v:=range dnsServersTable{
		if(v.Host == host){
			dnsServersTable[id].Errors = dnsServersTable[id].Errors + 1
			if(dnsServersTable[id].Errors > errorsLimit){
				//fmt.Printf("CANCELING THIS SERVER: %v || errors: %v\n", host, dnsServersTable[id].Errors)
				dnsServersTable[id].Status = false
			}	
		}
	}	
}

//ValidateDNSServers func
func validateDNSServers(servers []string) []string{
	var res []string

	for _, value := range servers{		
		if(len(value)< 1){
			continue
		}

		s := strings.Split(value, ":")
		if(len(s) > 1){
			res = append(res, value)
		} else{
			res = append(res, value+":53")
		}
	}

	return res
}

//ReturnDNSServersSlice desc
func ReturnDNSServersSlice()[]string{
	var s []string
	for _,v:=range dnsServersTable{
		s=append(s, v.Host)
	}

	return s
}

//PrintDNSServerList desc
func PrintDNSServerList(){
	var separator = " ----------------------------------------- "

	fmt.Println("")
	fmt.Println(separator)
	fmt.Println(" | DNS Server List                        |")
	fmt.Println(separator)
    fmt.Println(" |                   ip | errors | status |")
	fmt.Println(separator)

	for _,v:=range dnsServersTable{
		fmt.Printf(" | %20s | %6v | %6v |\n", v.Host, v.Errors, v.Status)
	}
	
	fmt.Println(separator)
}