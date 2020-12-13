package tables

import (
	
	"strings"
	"strconv"
		
	util "github.com/bp0lr/dmut/util"
)

//GenerateTables desc
func GenerateTables(trd string, alterations []string) []string{

	var res []string
	
	AddToDomain(trd, alterations, &res)
	AddNumbers(trd, &res)
	AddSeparator(trd, alterations, &res)
	
	//removing duplicated from job.tasks
	res = util.RemoveDuplicatesSlice(res)

	return res
}

//AddToDomain desc
func AddToDomain(trd string, alterations []string, res *[]string){

	//	this will add each alteration to the existing domain.
	//	for example to test some.test.com we are going to generate alt1.some.test.com and some.alt1.test.com
	///////////////////////////////////////////////////////////////////////////////////////////////			
	for _, alt := range alterations {

		if(len(alt) < 1){
			continue
		}

		strSplit := strings.Split(trd, ".")

		for i := 0; i <= len(strSplit); i++ {
			val:=util.Insert(strSplit, i, alt)
			*res = append(*res, strings.Join(val, "."))
		}
	}
}	

//AddNumbers desc
func AddNumbers(trd string, res *[]string){

	//	this will add a number to the end of each subdmain part.
	//	for example to test some.test.com we are going to generate some1.some.test.com, some2.alt1.test.com, etc
	///////////////////////////////////////////////////////////////////////////////////////////////		
	for index := 0; index < 10; index++ {		
		strSplit := strings.Split(trd, ".")

		for i := 0; i < len(strSplit); i++ {
			
			strclean:=strSplit[i]
			strSplit[i] = strclean+"-"+strconv.Itoa(index)
			*res = append(*res, strings.Join(strSplit, "."))
			
			strSplit[i] = strclean+strconv.Itoa(index)
			*res = append(*res, strings.Join(strSplit, "."))
		}
	}
}	

//AddSeparator desc
func AddSeparator(trd string, alterations []string, res *[]string){

	//	this will add (clean and using a -) each alteration to each subdomain part.
	//	for example to test some.test.com we are going to generate some-alt1.test.com, alt1-some.test.com, etc
	///////////////////////////////////////////////////////////////////////////////////////////////
	for _, alt := range alterations {
		if(len(alt) < 1){
			continue
		}

		strSplit := strings.Split(trd, ".")

		for i := 0; i < len(strSplit); i++ {			
			strclean:=strSplit[i]

			strSplit[i] = strclean+"-"+alt
			*res = append(*res, strings.Join(strSplit, "."))

			strSplit[i] = alt+"-"+strclean
			*res = append(*res, strings.Join(strSplit, "."))

			strSplit[i] = strclean+alt
			*res = append(*res, strings.Join(strSplit, "."))

			strSplit[i] = alt+strclean
			*res = append(*res, strings.Join(strSplit, "."))
		}
	}
}