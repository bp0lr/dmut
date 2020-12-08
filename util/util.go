package util

import (
	"io"
	"os"
	"os/user"
	"path/filepath"
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

//DownloadResolverList func
func DownloadResolverList() (string, error){
	
	dir,err := GetDir()
	if(err != nil){
		return "", err
	}
	os.MkdirAll(dir, 0644)

	fullPath := filepath.Join(dir, "resolvers.txt")

	os.Remove(fullPath)
	out, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	resp, err := http.Get("https://raw.githubusercontent.com/bp0lr/dmut-resolvers/main/resolvers.txt")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return fullPath, nil
}

//GetDir desc
func GetDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	
	return filepath.Join(usr.HomeDir, ".dmut"), nil
}