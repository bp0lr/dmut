//
// @bp0lr - 30/11/2020
//

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"

	def "github.com/bp0lr/dmut/defines"
	dnsManager "github.com/bp0lr/dmut/dnsManager"
	resolver "github.com/bp0lr/dmut/resolver"
	tables "github.com/bp0lr/dmut/tables"
	util "github.com/bp0lr/dmut/util"

	flag "github.com/spf13/pflag"
	tld "github.com/weppos/publicsuffix-go/publicsuffix"

	miekg "github.com/miekg/dns"

	pb "github.com/cheggaaa/pb/v3"
)

var (
		workersArg				int
		dnsRetriesArg			int
		dnsTimeOutArg			int
		dnserrorLimitArg		int
		mutationsDic			string
		urlArg					string
		outputFileArg			string
		dnsFileArg				string
		dnsArg					string
		verboseArg				bool
		updateDNSArg			bool
		updateNeededFIlesArg	bool
		ipArg					bool
		statsArg				bool
		pbArg					bool
		saveAndExitArg			bool
		dnsServers				[]string
		PermutationListArg		def.PermutationList	
)


//GlobalStats desc
var GlobalStats def.Stats

//GlobalLoadStats desc
var GlobalLoadStats def.LoadStats

var bar *pb.ProgressBar

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
	flag.BoolVar(&pbArg, "use-pb", false, "use a progress bar")
	flag.BoolVar(&saveAndExitArg, "save-gen", false, "save generated permutations to a file and exit")

	flag.BoolVar(&PermutationListArg.AddToDomain, "disable-permutations", false, "Disable permutations generation")
	flag.BoolVar(&PermutationListArg.AddNumbers, "disable-addnumbers", false, "Disable add numbers generation")
	flag.BoolVar(&PermutationListArg.AddSeparator, "disable-addseparator", false, "Disable add separator generation")
	
	flag.BoolVar(&updateNeededFIlesArg, "update-files", false, "Download all the default files to work with dmut. (default mutation list, resolvers, etc)")

	flag.Parse()

	if(updateDNSArg || updateNeededFIlesArg){
		dnsListTxt, errDNSListTxt:=util.DownloadFile("https://raw.githubusercontent.com/bp0lr/dmut-resolvers/main/resolvers.txt", "resolvers.txt")
		if(errDNSListTxt != nil){
			fmt.Printf("[-] Error: %v\n", errDNSListTxt)
		}else{
			fmt.Printf("[+] Location: %v\n", dnsListTxt)
		}

		dnsListTopTxt, errDNSListTopTxt:=util.DownloadFile("https://raw.githubusercontent.com/bp0lr/dmut-resolvers/main/top20.txt", "top20.txt")
		if(errDNSListTopTxt != nil){
			fmt.Printf("[-] Error: %v\n", errDNSListTopTxt)
		}else{
			fmt.Printf("[+] Location: %v\n", dnsListTopTxt)
		}

		mutationListTxt, errMutationTxt:=util.DownloadFile("https://raw.githubusercontent.com/bp0lr/dmut/main/words.txt", "words.txt")
		if(errMutationTxt != nil){
			fmt.Printf("[-] Error: %v\n", errMutationTxt)
		}else{
			fmt.Printf("[+] Location: %v\n", mutationListTxt)
		}
		
		fmt.Print("[+] download finished\n")
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
		  }
		
		content, _ := ioutil.ReadFile(dnsFileArg)
		dnsServers = strings.Split(string(content), "\n")		  
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
		}
		
		mutations, _ := ioutil.ReadFile(mutationsDic)
		alterations = strings.Split(string(mutations), "\n")		
	}

	GlobalStats.Mutations = len(alterations)

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

	GlobalStats.Domains = len(jobs)

	///////////////////////////////
	// generate taks list
	///////////////////////////////
	targetDomains := make(chan string)
	var wg sync.WaitGroup
	var mu = &sync.Mutex{}

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			for task := range targetDomains {				
				sl, _ :=generateTable(task, alterations, dnsTimeOutArg, dnsRetriesArg, dnserrorLimitArg, PermutationListArg)
				if(sl != nil){
					mu.Lock()
					GlobalStats.WorksToDo = append(GlobalStats.WorksToDo, sl...)
					mu.Unlock()			
				}
			}
			wg.Done()
		}()
	}
		
	for _, line := range jobs {
		targetDomains <- line
	}
	
	close(targetDomains)	
	wg.Wait()	

	if(saveAndExitArg){
		var file, err = os.Create("generated.txt")
		if err != nil {
			fmt.Printf("Error saving generated.txt\n%v\n", err)
        	return
    	}
    	defer file.Close()

		file.WriteString(fmt.Sprintln(strings.Join(GlobalStats.WorksToDo, "\n")))
    
    	file.Sync()

		fmt.Printf("Permutation list saved to generated.txt\n")
		os.Exit(0)
	}

	///////////////////////////////
	// Lets process the task list
	///////////////////////////////
	if(verboseArg){
		fmt.Printf("total Domains: %v | valid: %v | errors: %v\n", GlobalLoadStats.Domains, GlobalLoadStats.Valid, GlobalLoadStats.Errors)
		fmt.Printf("Works to do: %v\n", len(GlobalStats.WorksToDo))
	}
	
	if(pbArg){
		tmpl := `{{ red "Mutations:" }} {{counters . | red}}  {{ bar . "<" "-" (cycle . "↖" "↗" "↘" "↙" ) "." ">"}} {{percent .}} [{{speed . | green }}] [{{rtime . "ETA %s"}}]`
		bar = pb.ProgressBarTemplate(tmpl).Start(len(GlobalStats.WorksToDo))
		//bar = pb.Full.Start(len(GlobalStats.WorksToDo))
		bar.SetWidth(100)
	}
	
	tasks := make(chan string)
	var wg2 sync.WaitGroup
	
	for i := 0; i < workers; i++ {
		wg2.Add(1)
		go func() {
			for task := range tasks {				
				//fullDomain:= task + "." + job.sld + "." + job.tld + "."
				processDNS(task, outputFile, dnsTimeOutArg, dnsRetriesArg, dnserrorLimitArg)
			}
			wg2.Done()
		}()
	}
		
	for _, line := range  GlobalStats.WorksToDo {
		tasks <- line
	}
	
	close(tasks)	
	wg2.Wait()	
		
	///////////////////////////////
	// Lets print the results
	///////////////////////////////
	
	if(pbArg){
		bar.Finish()
	}

	if(len(GlobalStats.FoundDomains) > 0 && pbArg){
		fmt.Println("")
		for _,v:=range GlobalStats.FoundDomains{
			fmt.Println(v)
		}
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
		fmt.Printf(" | Domains:      %24v | \n", GlobalStats.Domains)
		fmt.Printf(" | mutations:    %24v | \n", GlobalStats.Mutations)
		fmt.Printf(" | Queries:      %24v | \n", len(GlobalStats.WorksToDo))
		fmt.Printf(" | Sub Found:    %24v | \n", GlobalStats.Founds)
		fmt.Printf(" -----------------------------------------\n")
	}
}

func generateTable(domain string, alterations [] string, dnsTimeOut int, dnsRetries int, dnsErrorLimit int, permutationList def.PermutationList) ([]string, error){
	
	GlobalLoadStats.Domains++

	_, err := url.Parse(domain)
	if err != nil {
		if verboseArg {
			fmt.Printf("[-] Invalid url: %s\n", domain)
		}
		GlobalLoadStats.Errors++
		return nil, err
	}		
	
	domParse, err :=tld.Parse(domain)
	if(err != nil){
		if(verboseArg){
			fmt.Printf("[-] Error parsing url: %s\n", domain)
		}

		GlobalLoadStats.Errors++
		return nil, err
	}
	
	var job def.DmutJob
	job.Domain 	= domain					//domain: lala.test.val.redbull.com
	job.Sld 	= domParse.SLD				//sld: redbull 
	job.Tld 	= domParse.TLD 				//tld: com
	job.Trd 	= domParse.TRD				//trd: lala.test.val
	

	//testing if domain response like wildcard. If this is the case, we cancel this task.
	if(!saveAndExitArg){
		if(checkWildCard(job, dnsTimeOut, dnsRetries, dnsErrorLimit)){
			if(verboseArg){
				fmt.Printf("[%v] dns wild card detected. Canceling this job!\n", domain)
			}
			GlobalLoadStats.Errors++
			return nil, nil
		}
	}

	tmp:= tables.GenerateTables(job, alterations, permutationList)

	GlobalLoadStats.Valid++

	return tmp, nil
}

func processDNS(domain string, outputFile *os.File, dnsTimeOut int, dnsRetries int, dnsErrorLimit int) {

	if verboseArg {
		fmt.Printf("[+] Testing: %v\n", domain)
	}

	qtype:= []uint16 {miekg.TypeA, miekg.TypeCNAME}
	// I prefer to make a single query for each type, stop it if a have found something to win some milliseconds.
	for _, value := range qtype{		
		
		//fmt.Printf("[%v] send type %v\n", domain, value)
		resp, err := resolver.GetDNSQueryResponse(domain, value, dnsTimeOut, dnsRetries, dnsErrorLimit, "")
		if(verboseArg){
			if(err != nil){
			fmt.Printf("err resolver: %v\n", err)
			}
		}

		if(!resp.Status){
			continue
		}

		found:=processResponse(domain, resp, outputFile, value)
		if(found){
			break
		}
	}
	
	if(pbArg){
		bar.Increment()
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
		if(verboseArg){
			if(reErr != nil){
			//fmt.Printf("err resolver: %v\n", reErr)
			}
		}

		if(retest.Status){
			if(retest.Data.StatusCode != "NOERROR"){

				//fmt.Printf("[%v] ban by goole retest  %v <> %v\n", domain, result.Data.StatusCode, retest.Data.StatusCode)
				//fmt.Printf("The domain %v was reported like OK, but can't pass the retest again google.\n", domain)
				//fmt.Printf("Verboise: %v\n", retest.Data.StatusCode)
				return false
			}
			
			//fmt.Printf("[%v] confirm by goole retest  %v <> %v\n", domain, result.Data.StatusCode, retest.Data.StatusCode)
			//fmt.Printf("domain %v pass the re test!\n", domain)				
			//fmt.Printf("Verboise: %v\n", retest.Data.StatusCode)
			//fmt.Printf("Verboise: %v\n", retest.Data.OriRes)		
		}else{			
			return false
		}
						
		if outputFileArg != "" {
			if(ipArg){	
				outputFile.WriteString(trimDomain + ":" + util.TrimChars(strings.Join(result.Data.CNAME,",")) + util.TrimChars(strings.Join(result.Data.A,",")) + "\n")
			} else {
				outputFile.WriteString(trimDomain + "\n")
			}
		}	
		if(!pbArg){
			if(ipArg){
				fmt.Printf("%v : [%v]:[%v]\n", trimDomain, util.TrimChars(strings.Join(result.Data.CNAME,",")) , util.TrimChars(strings.Join(result.Data.A,",")))
			}else{
				fmt.Printf("%v\n", trimDomain)
			}
		}else{ 
			GlobalStats.FoundDomains=append(GlobalStats.FoundDomains, trimDomain)
		}

		GlobalStats.Founds++

		return true
	}

	return false
}

func checkWildCard(job def.DmutJob, dnsTimeOut int, dnsRetries int, dnsErrorLimit int)bool{
	
	var res resolver.JobResponse
	var err error
	
	var mutations []string
	var wildcard bool = false
	var text = []string{"supposedtonotexistmyfriend"}

	tables.AddToDomain(job, text, &mutations)

	for _, mutation := range mutations{
		//fmt.Printf("[%v] trying %v\n", job.Domain, mutation)
		res, err = resolver.GetDNSQueryResponse(mutation, miekg.TypeA, dnsTimeOut, dnsRetries, dnsErrorLimit, "8.8.8.8:53")
		if(err != nil){
			//fmt.Printf("antiWild reported err: %v\n", err)
			continue
		}

		if(res.Status && res.Data.StatusCode != "NXDOMAIN"){
			//fmt.Printf("[%v] Wilcard Positive response. Canceling tasks: %v\n", job.Domain, mutation)
			wildcard = true
			break
		}else{
			//fmt.Printf("[%v] Wilcard Positive response OK. %v\n", job.Domain, res.Data.StatusCode)
			wildcard = false
		}
	}

	return wildcard
}