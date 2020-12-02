package util

import (
	"io"
	"os"
	"strings"
	"net/http"	
)

//Insert func
func Insert(a []string, index int, value string) []string {
    if len(a) == index { 
        return append(a, value)
    }
    a = append(a[:index+1], a[index:]...)
    a[index] = value
    return a
}

//TrimLastPoint func
func TrimLastPoint(s, suffix string) string {
    if strings.HasSuffix(s, suffix) {
        s = s[:len(s)-len(suffix)]
    }
    return s
}

//RemoveDuplicatesSlice func
func RemoveDuplicatesSlice(s []string) []string {
	m := make(map[string]bool)
	for _, item := range s {
			if _, ok := m[item]; ok {
					//fmt.Printf("Duplicate: %v\n", item)
			} else {
					m[item] = true
			}
	}

	var result []string
	for item := range m {
		result = append(result, item)
	}

	return result
}

//TrimChars func
func TrimChars(s string) string {
	return strings.TrimRight(s, ".")
}

//ValidateDNSServers func
func ValidateDNSServers(servers []string) []string{

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

//DownloadResolverList func
func DownloadResolverList() error{
		
	out, err := os.Create("resolvers.txt")
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get("https://raw.githubusercontent.com/janmasarik/resolvers/master/resolvers.txt")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}