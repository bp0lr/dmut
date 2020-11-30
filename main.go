//
// @bp0lr - 30/11/2020
//

package main

import (
	"os"
	"fmt"
	//"net"
	"sync"
	//"time"
	"bufio"
	"strings"
	//"strconv"
	"net/url"
	//"net/http"
	//"math/rand"
	//"crypto/tls"
	
	//"encoding/json"

	resolver	"github.com/bp0lr/dmut/resolver"

	flag 		"github.com/spf13/pflag"
	tld 		"github.com/weppos/publicsuffix-go/publicsuffix"
	//			"github.com/peterzen/goresolver"

)

var (
	workersArg	      	int
	mutationsDic		string
	urlArg            	string
	outputFileArg     	string
	verboseArg        	bool
)

//jobL desc
type jobL struct {
	tld        	string
	sld  		string
	trd  		string
	tasks		[]string
	//var2		[]string
	//var3		[]string
}

func main() {

	flag.IntVarP(&workersArg, "workers", "w", 50, "Workers amount")
	flag.StringVarP(&urlArg, "url", "u", "", "The firebase url to test")
	flag.BoolVarP(&verboseArg, "verbose", "v", false, "Display extra info about what is going on")
	flag.StringVarP(&mutationsDic, "dic", "d", "", "Dictionary file containing mutation list")
	flag.StringVarP(&outputFileArg, "output", "o", "", "Output file to save the results to")
	
	flag.Parse()

	//concurrency
	workers := 200
	if workersArg > 0  && workersArg < 400 {
		workers = workersArg
	}

	fmt.Printf("workers: %v\n", workers)

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

	alterations := []string{"dev", "q", "staging", "stage"}
		
	for _, value := range jobs {
		processDomain(workers, value, alterations, outputFile)
	}

}

func processDomain(workers int, domain string, alterations [] string, outputFile *os.File){

	_, err := url.Parse(domain)
	if err != nil {
		if verboseArg {
			fmt.Printf("[-] Invalid url: %s\n", domain)
		}
		return
	}		
	
	domParse, _:=tld.Parse(domain)
	
	//tld: com
	//sld: redbull
	//trd: lala.test.val

	var job jobL	
	job.sld = domParse.SLD
	job.tld = domParse.TLD
	job.trd = domParse.TRD
	
	//	this will add each alteration to the existing domain.
	//	for example to test some.test.com we are going to generate alt1.some.test.com and some.alt1.test.com
	///////////////////////////////////////////////////////////////////////////////////////////////			
	for _, alt := range alterations {
		strSplit := strings.Split(job.trd, ".")

		for i := 0; i < len(strSplit); i++ {
			val:=insert(strSplit, i, alt)
			job.tasks = append(job.tasks, strings.Join(val, "."))
		}
	}
	
	//fmt.Printf("%v\n", job.var1)

	//	this will add a number to the end of each subdmain part.
	//	for example to test some.test.com we are going to generate some1.some.test.com, some2.alt1.test.com, etc
	///////////////////////////////////////////////////////////////////////////////////////////////		
	/*
	for index := 0; index < 10; index++ {		
		strSplit := strings.Split(job.trd, ".")

		for i := 0; i < len(strSplit); i++ {
			
			strclean:=strSplit[i]
			strSplit[i] = strclean+"-"+strconv.Itoa(index)
			job.tasks = append(job.tasks, strings.Join(strSplit, "."))
			
			strSplit[i] = strclean+strconv.Itoa(index)
			job.tasks = append(job.tasks, strings.Join(strSplit, "."))
		}
	}
	*/
	//fmt.Printf("strSplit: %v\n", job.var2)
	
	//	this will add (clean and using a -) each alteration to each subdomain part.
	//	for example to test some.test.com we are going to generate some-alt1.test.com, alt1-some.test.com, etc
	///////////////////////////////////////////////////////////////////////////////////////////////
	for _, alt := range alterations {
		strSplit := strings.Split(job.trd, ".")

		for i := 0; i < len(strSplit); i++ {			
			strclean:=strSplit[i]

			strSplit[i] = strclean+"-"+alt
			job.tasks = append(job.tasks, strings.Join(strSplit, "."))

			strSplit[i] = alt+"-"+strclean
			job.tasks = append(job.tasks, strings.Join(strSplit, "."))

			strSplit[i] = strclean+alt
			job.tasks = append(job.tasks, strings.Join(strSplit, "."))

			strSplit[i] = alt+strclean
			job.tasks = append(job.tasks, strings.Join(strSplit, "."))
		}
	}
	
	//fmt.Printf("strSplit: %v\n", job.var3)

	fmt.Printf("[%v] We have %v jobs to do.\n", domain, len(job.tasks))
	
	//jobs := make(chan string)
	var wg sync.WaitGroup

	//for i := 0; i < workers; i++ {
	//	wg.Add(1)
	//	go func() {
			for _, task := range job.tasks {			
				fullDomain:= task + "." + job.sld + "." + job.tld + "."
				fmt.Printf("working on %v\n", fullDomain)
				processDNS(&wg, fullDomain, outputFile)
			}
	//		wg.Done()			
	//	}()
	//}

	//close(jobs)	
	//wg.Wait()
}

func processDNS(wg *sync.WaitGroup, domain string, outputFile *os.File) {

	//if verboseArg {
		fmt.Printf("[+] Testing: %v\n", domain)
	//}

	//result, err := resolver.LookupIP(domain)
	result, err := resolver.GetDNSQueryResponse(5, domain, "8.8.8.8")
	
	if err != nil {
		//fmt.Printf("error %v is invalid", domain)
	}else{
		fmt.Printf("%v : %v\n", domain, result)
	}
		
	//if outputFileArg != "" {
	//	outputFile.WriteString(u.String() + "\n")
	//}	
}



func insert(a []string, index int, value string) []string {
    if len(a) == index { // nil or empty slice or after last element
        return append(a, value)
    }
    a = append(a[:index+1], a[index:]...) // index < len(a)
    a[index] = value
    return a
}