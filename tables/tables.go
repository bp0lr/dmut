package tables

import (
	//"os"
	//"fmt"	
	"strings"
	"strconv"
	
	def 	"github.com/bp0lr/dmut/defines"
	util 	"github.com/bp0lr/dmut/util"
)

type dmutJob = def.DmutJob

//GenerateTables desc
func GenerateTables(job def.DmutJob, alterations []string) []string{

	var res []string
	
	AddToDomain(job, alterations, &res)
	AddNumbers(job, &res)
	AddSeparator(job, alterations, &res)
	
	//removing duplicated from job.tasks
	res = util.RemoveDuplicatesSlice(res)

	return res
}

//AddToDomain desc
func AddToDomain(job dmutJob, alterations []string, res *[]string){

	//	this will add each alteration to the existing domain.
	//	for example to test some.test.com we are going to generate alt1.some.test.com and some.alt1.test.com
	///////////////////////////////////////////////////////////////////////////////////////////////			
	for _, alt := range alterations {

		if(len(alt) < 1){
			continue
		}
		
		strSplit := strings.Split(job.Trd, ".")

		for i := 0; i <= len(strSplit); i++ {
			val:=util.Insert(strSplit, i, alt)
			fullDomain:= strings.Join(val, ".") + "." + job.Sld + "." + job.Tld
			//fmt.Printf("val: %v\n", fullDomain)
			*res = append(*res, fullDomain)
		}
	}
}	

//AddNumbers desc
func AddNumbers(job def.DmutJob, res *[]string){

	//	this will add a number to the end of each subdmain part.
	//	for example to test some.test.com we are going to generate some1.some.test.com, some2.alt1.test.com, etc
	///////////////////////////////////////////////////////////////////////////////////////////////		
	for index := 0; index < 10; index++ {		
		strSplit := strings.Split(job.Trd, ".")

		var fullDomain string			
		for i := 0; i < len(strSplit); i++ {			
			strclean:=strSplit[i]
			strSplit[i] = strclean+"-"+strconv.Itoa(index)
			fullDomain= strings.Join(strSplit, ".") + "." + job.Sld + "." + job.Tld
			*res = append(*res, fullDomain)
			
			strSplit[i] = strclean+strconv.Itoa(index)
			fullDomain= strings.Join(strSplit, ".") + "." + job.Sld + "." + job.Tld
			*res = append(*res, fullDomain)
		}
	}
}	

//AddSeparator desc
func AddSeparator(job def.DmutJob, alterations []string, res *[]string){

	//	this will add (clean and using a -) each alteration to each subdomain part.
	//	for example to test some.test.com we are going to generate some-alt1.test.com, alt1-some.test.com, etc
	///////////////////////////////////////////////////////////////////////////////////////////////
	for _, alt := range alterations {
		if(len(alt) < 1){
			continue
		}

		var fullDomain string

		strSplit := strings.Split(job.Trd, ".")				

		for i := 0; i < len(strSplit); i++ {			
			strclean:=strSplit[i]
			
			strSplit[i] = strclean+"-"+alt
			fullDomain= strings.Join(strSplit, ".") + "." + job.Sld + "." + job.Tld
			*res = append(*res, fullDomain)

			strSplit[i] = alt+"-"+strclean
			fullDomain= strings.Join(strSplit, ".") + "." + job.Sld + "." + job.Tld
			*res = append(*res, fullDomain)

			strSplit[i] = strclean+alt
			fullDomain= strings.Join(strSplit, ".") + "." + job.Sld + "." + job.Tld
			*res = append(*res, fullDomain)

			strSplit[i] = alt+strclean
			fullDomain= strings.Join(strSplit, ".") + "." + job.Sld + "." + job.Tld
			*res = append(*res, fullDomain)
		}
	}	
}