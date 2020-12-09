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
	
	dnsManager	"github.com/bp0lr/dmut/dnsManager"
	util		"github.com/bp0lr/dmut/util"
	tables		"github.com/bp0lr/dmut/tables"
	resolver	"github.com/bp0lr/dmut/resolver"
	
	flag 		"github.com/spf13/pflag"
	tld 		"github.com/weppos/publicsuffix-go/publicsuffix"
	
	miekg		"github.com/miekg/dns"
)

var (
	workersArg	      	int
	dnsRetriesArg		int
	dnsTimeOutArg		int
	dnserrorLimitArg	int
	mutationsDic		string
	urlArg				string
	outputFileArg		string
	dnsFileArg			string
	dnsArg				string
	verboseArg			bool
	updateDNSArg		bool
	ipArg				bool
	statsArg			bool
	dnsServers			[]string	
)

//jobL desc
type jobL struct {
	tld        	string
	sld  		string
	trd  		string
	tasks		[]string
}

type stats struct{
	queries	int
	domains int
	mutations int
	founds	int
}

//GlobalStats desc
var GlobalStats stats

func main() {

	var alterations []string

	flag.IntVarP(&workersArg, "workers", "w", 25, "Number of workers")
	flag.StringVarP(&urlArg, "url", "u", "", "Target URL")
	flag.BoolVarP(&verboseArg, "verbose", "v", false, "Add verboicity to the process")
	flag.StringVarP(&mutationsDic, "dictionary", "d", "", "Dictionary file containing mutation list")
	flag.StringVarP(&outputFileArg, "output", "o", "", "Output file to save the results to")
	flag.StringVarP(&dnsFileArg, "dnsFile", "s", "", "Use DNS servers from this file")
	flag.StringVarP(&dnsArg, "dnsServers", "l", "", "Use DNS servers from a list separated by ,")
	flag.BoolVar(&updateDNSArg, "update-dnslist", false, "Download a list of periodically validated public DNS resolvers")
	flag.BoolVar(&ipArg, "show-ip", false, "Display info for valid results")
	flag.BoolVar(&statsArg, "show-stats", false, "Display stats about the current job")
	flag.IntVar(&dnsRetriesArg, "dns-retries", 3, "Amount of retries for failed dns queries")
	flag.IntVar(&dnsTimeOutArg, "dns-timeout", 500, "Dns Server timeOut in millisecond")
	flag.IntVar(&dnserrorLimitArg, "dns-errorLimit", 25, "How many errors until we the DNS is disabled")

	flag.Parse()

	if(updateDNSArg){
		file, err:=util.DownloadFile("https://raw.githubusercontent.com/bp0lr/dmut-resolvers/main/resolvers.txt", "resolvers.txt")
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
	if workersArg > 0  &&  workersArg < 151 {
		workers = workersArg
	}else{
		fmt.Printf("[+] Workers amount should be between 1 and 150.\n")
		fmt.Printf("[+] The number of workers was set to 25.\n")		
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

	dnsManager.GenerateDNSServersTable(dnsServers);
			
	if(verboseArg){
		fmt.Printf("[+] we are using this dns servers: %v\n", dnsManager.ReturnDNSServersSlice())
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

	GlobalStats.mutations = len(alterations)

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

	GlobalStats.domains = len(jobs)

	for _, value := range jobs {
		processDomain(workers, value, alterations, outputFile, dnsTimeOutArg, dnsRetriesArg, dnserrorLimitArg)
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

	if(statsArg){
		dnsManager.PrintDNSServerList()
		fmt.Printf(" | Domains:      %24v | \n", GlobalStats.domains)
		fmt.Printf(" | mutations:    %24v | \n", GlobalStats.mutations)
		fmt.Printf(" | Queries:      %24v | \n", GlobalStats.queries)
		fmt.Printf(" | Sub Found:    %24v | \n", GlobalStats.founds)
		fmt.Printf(" -----------------------------------------\n")
	}
	
}

func processDomain(workers int, domain string, alterations [] string, outputFile *os.File, dnsTimeOut int, dnsRetries int, dnsErrorLimit int){

	_, err := url.Parse(domain)
	if err != nil {
		if verboseArg {
			fmt.Printf("[-] Invalid url: %s\n", domain)
		}
		return
	}		
	
	//testing if domain response to wil card. If this is the case, we cancel this task.
	if(checkWildCard(domain, dnsTimeOut, dnsRetries, dnsErrorLimit)){
		if(verboseArg){
			fmt.Printf("[%v] dns wild card detected. Canceling this job!\n", domain)
		}
		return
	}
	
	domParse, _:=tld.Parse(domain)
	
	var job jobL	
	job.sld = domParse.SLD				//sld: redbull 
	job.tld = domParse.TLD 				//tld: com
	job.trd = domParse.TRD				//trd: lala.test.val
	
	job.tasks = tables.GenerateTables(job.trd, alterations)

	GlobalStats.queries = GlobalStats.queries + len(job.tasks)
	
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
				processDNS(&wg, fullDomain, outputFile, dnsTimeOut, dnsRetries, dnsErrorLimit)
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

func processDNS(wg *sync.WaitGroup, domain string, outputFile *os.File, dnsTimeOut int, dnsRetries int, dnsErrorLimit int) {

	if verboseArg {
		fmt.Printf("[+] Testing: %v\n", domain)
	}

	qtype:= []uint16 {miekg.TypeA, miekg.TypeCNAME}
	// I prefer to make a single query for each type, stop it if a have found something to win some milliseconds.
	for _, value := range qtype{		
		
		//fmt.Printf("[%v] send type %v\n", domain, value)
		resp, err := resolver.GetDNSQueryResponse(domain, value, dnsTimeOut, dnsRetries, dnsErrorLimit, "")
		if(err != nil){
			fmt.Printf("err resolver: %v\n", err)
		}

		if(!resp.Status){
			continue
		}

		found:=processResponse(domain, resp, outputFile, value)
		if(found){
			break
		}
	}	
}

func processResponse(domain string, result resolver.JobResponse, outputFile *os.File, qType uint16)bool{
	
	if (result.Status) {		
		
		trimDomain:= util.TrimLastPoint(domain, ".")
		if(result.Data.StatusCode != "NOERROR"){
			if(verboseArg){
				fmt.Printf("[%v] code: %v\n", domain, result.Data.StatusCode)
			}
			return false
		}

		if(qType == miekg.TypeCNAME){
			if(len(result.Data.CNAME) < 1){
				//fmt.Printf("A NOERROR was reported for domain %v but CNAME was empty.\n", domain)
				//fmt.Printf("raw: %v\n", result.Data.Raw)
				return false
			}	
			
			//fmt.Printf("[%v] reported a valid CNAME value for %v size: %v!\n", result.Data.Resolver, domain, len(result.Data.CNAME))
			//fmt.Printf("debug: %v\n", result.Data.CNAME)
		}

		if(qType == miekg.TypeA){
			if(len(result.Data.A) < 1){
				//fmt.Printf("A NOERROR was reported for domain %v but A was empty.\n", domain)
				//fmt.Printf("raw: %v\n", result.Data.Raw)
				return false
			}
			
			//fmt.Printf("[%v] reported a valid A value for %v size: %v!\n", result.Data.Resolver, domain, len(result.Data.A))
			//fmt.Printf("debug: %v\n", result.Data.A)
			
		}
				
		//re validated what we found again google dns just to be sure.
		retest, reErr := resolver.GetDNSQueryResponse(domain, qType, 500, 3, 10, "8.8.8.8:53")
		if(reErr != nil){
			fmt.Printf("err resolver: %v\n", reErr)
		}

		if(retest.Status){
			if(retest.Data.StatusCode != "NOERROR"){

				fmt.Printf("[%v] ban by goole retest  %v <> %v\n", domain, result.Data.StatusCode, retest.Data.StatusCode)
				//fmt.Printf("The domain %v was reported like OK, but can't pass the retest again google.\n", domain)
				//fmt.Printf("Verboise: %v\n", retest.Data.StatusCode)
				return false
			}
			
			fmt.Printf("[%v] confirm by goole retest  %v <> %v\n", domain, result.Data.StatusCode, retest.Data.StatusCode)
			//fmt.Printf("domain %v pass the re test!\n", domain)				
			//fmt.Printf("Verboise: %v\n", retest.Data.StatusCode)
			//fmt.Printf("Verboise: %v\n", retest.Data.OriRes)		
		}else{			
			return false
		}
		

				
		GlobalStats.founds = GlobalStats.founds + 1

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

		return true
	}

	return false
}

func checkWildCard(domain string, dnsTimeOut int, dnsRetries int, dnsErrorLimit int)bool{
	
	var res resolver.JobResponse
	var err error
	var wildcard bool = false

	var domains = []string{"supposedtonotexistmyfriend."+domain, "supposedtonotexistmyfriend"+domain}

	for _, v := range domains{
		//fmt.Printf("[%v] trying %v\n", domain, v)
		res, err = resolver.GetDNSQueryResponse(v, miekg.TypeA, dnsTimeOut, dnsRetries, dnsErrorLimit, "8.8.8.8:53")
		if(err != nil){
			//fmt.Printf("antiWild reported err: %v\n", err)
			continue
		}

		if(res.Status && res.Data.StatusCode != "NXDOMAIN"){
			//fmt.Printf("[%v] Wilcard Positive response. Canceling tasks.\n", domain)
			wildcard = true
			break
		}else{
			//fmt.Printf("[%v] Wilcard Positive response OK. %v\n", domain, res.Data.StatusCode)
			wildcard = false
		}
	}
	
	return wildcard
}