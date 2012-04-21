package main

import "net/http"
import "html/template"
import "flag"
import "os"
import "path"

var templateRoot = "/Users/mkb/code/opera-omnia/templates";

func init() {
	flag.StringVar(&templateRoot, "templates", templateRoot, "")
	http.HandleFunc("/", IndexHandler)
}

func IndexHandler(resp http.ResponseWriter, req *http.Request) {
	// get template
	p := req.URL.Path
	if p == "/" {
		p = "/index.html"
	}
	t, err := template.ParseFiles(path.Join(templateRoot, req.URL.Path))
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(resp, "Template Not Found", 404)
		} else {
			http.Error(resp, err.Error(), 500)
		}
		return
	}
	err = t.Execute(resp, req)
	if err != nil {
		http.Error(resp, err.Error(), 500)
	}
}

