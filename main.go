//
// @bp0lr - 30/11/2020
//

package main

import (
	"path"
	"os"
	"fmt"
	"sync"
	"bufio"
	"io/ioutil"
	"strings"
	"net/url"
		
	util		"github.com/bp0lr/dmut/util"
	tables		"github.com/bp0lr/dmut/tables"
	resolver	"github.com/bp0lr/dmut/resolver"

	flag 		"github.com/spf13/pflag"
	tld 		"github.com/weppos/publicsuffix-go/publicsuffix"
)

var (
	workersArg	      	int
	dnsRetriesArg		int
	dnsTimeOutArg		int
	mutationsDic		string
	urlArg            	string
	outputFileArg     	string
	dnsFileArg			string
	dnsArg				string
	verboseArg        	bool
	updateDNSArg		bool
	ipArg				bool
	dnsServers 			[]string	
)

//jobL desc
type jobL struct {
	tld        	string
	sld  		string
	trd  		string
	tasks		[]string
}

func main() {

	var alterations []string

	flag.IntVarP(&workersArg, "workers", "w", 25, "Workers amount")
	flag.StringVarP(&urlArg, "url", "u", "", "Target URL")
	flag.BoolVarP(&verboseArg, "verbose", "v", false, "Display extra info about what is going on")
	flag.StringVarP(&mutationsDic, "dictionary", "d", "", "Dictionary file containing mutation list")
	flag.StringVarP(&outputFileArg, "output", "o", "", "Output file to save the results to")
	flag.StringVarP(&dnsFileArg, "dnsFile", "s", "", "Use DNS servers from this file")
	flag.StringVarP(&dnsArg, "dnsServers", "l", "", "Use DNS servers from a list separated by ,")
	flag.BoolVar(&updateDNSArg, "update-dnslist", false, "Download a list of periodically validated public DNS resolvers")
	flag.BoolVar(&ipArg, "show-ip", false, "Display info for valid results")
	flag.IntVar(&dnsRetriesArg, "dns-retries", 3, "Amount of retries for failed dns queries")
	flag.IntVar(&dnsTimeOutArg, "dns-timeout", 500, "Dns Server timeOut in millisecond")

	flag.Parse()

	if(updateDNSArg){
		file, err:=util.DownloadResolverList()
		if(err != nil){
			fmt.Printf("[-] Error: %v\n", err)
		}else{
			fmt.Printf("[+] File downloaded successfully!\n")
			fmt.Printf("[+] Location: %v\n", file)
		}
		return
	}

	//concurrency
	workers := 25
	if workersArg > 0  && workersArg < 100 {
		workers = workersArg
	}
	
	if(dnsTimeOutArg == 0 || dnsTimeOutArg > 10000) {
		dnsTimeOutArg = 500
	}

	if(verboseArg){
		fmt.Printf("[+] Workers: %v\n", workers)
	}

	if(len(dnsFileArg) > 0){
		if _, err := os.Stat(dnsFileArg); os.IsNotExist(err) {
			fmt.Printf("Error, %v\n", err)
			return
		  }else{
			content, _ := ioutil.ReadFile(dnsFileArg)
			dnsServers = strings.Split(string(content), "\n")
		  }
	}

	if(len(dnsArg) > 0){
		dnsServers = strings.Split(dnsArg, ",")
	}

	if(len(dnsServers) == 0){
		userDir, err:=util.GetDir()
		if err != nil{
			userDir=""
		}

		fullPath:= path.Join(userDir, "resolvers.txt")
		if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
			content, _ := ioutil.ReadFile(fullPath)
			dnsServers = strings.Split(string(content), "\n")
		}else{
			dnsServers = []string{
				"1.1.1.1:53", // Cloudflare
				"1.0.0.1:53", // Cloudflare
				"8.8.8.8:53", // Google
				"8.8.4.4:53", // Google
				"9.9.9.9:53", // Quad9
			}
		}
	}

	dnsServers = util.ValidateDNSServers(dnsServers)

	if(verboseArg){
		fmt.Printf("[+] Using dns server: %v\n", dnsServers)
		fmt.Printf("[+] Current dns timeout is: %v\n", dnsTimeOutArg)
	}
	
	if(len(mutationsDic) == 0){
		fmt.Printf("Error, you need to define a mutation file list using the arg -d\n")
		return
	}

	if(len(mutationsDic) > 0){
		if _, err := os.Stat(mutationsDic); os.IsNotExist(err) {
			fmt.Printf("Error, %v\n", err)
			return
		}else{
			mutations, _ := ioutil.ReadFile(mutationsDic)
			alterations = strings.Split(string(mutations), "\n")
		}
	}

	if(len(dnsServers) > 0 ){
		if(verboseArg){
			fmt.Printf("[+] we are using this dns servers: %v\n", dnsServers)
		}
	}

	var outputFile *os.File
	var err0 error
	if outputFileArg != "" {
		outputFile, err0 = os.OpenFile(outputFileArg, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
		if err0 != nil {
			fmt.Printf("cannot write %s: %s", outputFileArg, err0.Error())
			return
		}
		
		defer outputFile.Close()
	}

	var jobs []string

	if len(urlArg) < 1 {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			jobs = append(jobs, sc.Text())
		}
	} else {
		jobs = append(jobs, urlArg)
	}

	for _, value := range jobs {
		processDomain(workers, value, alterations, outputFile, dnsTimeOutArg, dnsRetriesArg)
	}

	//if we didn't found anything, delete the result file.
	if(len(outputFileArg) > 0){
		fi, err := os.Stat(outputFileArg)
		if err == nil {
			if(fi.Size() == 0){
				os.Remove(outputFileArg)
			}
		}
	}	
}

func processDomain(workers int, domain string, alterations [] string, outputFile *os.File, dnsTimeOut int, dnsRetries int){

	_, err := url.Parse(domain)
	if err != nil {
		if verboseArg {
			fmt.Printf("[-] Invalid url: %s\n", domain)
		}
		return
	}		
	
	domParse, _:=tld.Parse(domain)
	
	var job jobL	
	job.sld = domParse.SLD				//sld: redbull 
	job.tld = domParse.TLD 				//tld: com
	job.trd = domParse.TRD				//trd: lala.test.val
	
	job.tasks = tables.GenerateTables(job.trd, alterations)
	
	if(verboseArg){
		fmt.Printf("[%v] We have %v jobs to do.\n", domain, len(job.tasks))
	}
	
	jobs := make(chan string)
	var wg sync.WaitGroup
	
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			for task := range jobs {				
				fullDomain:= task + "." + job.sld + "." + job.tld + "."
				processDNS(&wg, fullDomain, outputFile, dnsTimeOut, dnsRetries)
			}
			wg.Done()
		}()
	}
		
	for _, line := range job.tasks {
		jobs <- line
	}
	
	close(jobs)	
	wg.Wait()	
}

func processDNS(wg *sync.WaitGroup, domain string, outputFile *os.File, dnsTimeOut int, dnsRetries int) {

	trimDomain:= util.TrimLastPoint(domain, ".")

	if verboseArg {
		fmt.Printf("[+] Testing: %v\n", domain)
	}

	result, err:= resolver.GetDNSQueryResponse(domain, dnsServers, dnsTimeOut, dnsRetries)
	
	
	if (err == nil && result.Status) {		
		fmt.Printf("---------------------------------\nstatus: %v\n", result.Data.StatusCode)
		fmt.Printf("%v\n---------------------------------\n", result)
		
		if outputFileArg != "" {
			if(ipArg){	
				outputFile.WriteString(trimDomain + ":" + util.TrimChars(strings.Join(result.Data.CNAME,",")) + util.TrimChars(strings.Join(result.Data.A,",")) + "\n")
			} else {
				outputFile.WriteString(trimDomain + "\n")
			}
		}	
		if(ipArg){
			fmt.Printf("%v : [%v]:[%v]\n", trimDomain, util.TrimChars(strings.Join(result.Data.CNAME,",")) , util.TrimChars(strings.Join(result.Data.A,",")))
		}else{
			fmt.Printf("%v\n", trimDomain)
		}
	} else{
		if(verboseArg){
			fmt.Printf("status blank err: %v\n", err)
		}
	}
}