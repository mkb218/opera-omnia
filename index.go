package main

import "log"
import "net/http"
import "html/template"
//import "flag"
import "os"
import "path"
import "strings"

func init() {
	http.HandleFunc("/", IndexHandler)
}

func IndexHandler(resp http.ResponseWriter, req *http.Request) {
	// get template
	p := req.URL.Path
	if p == "/" || p == "" {
		p = "/index.html"
	}
	p = path.Join(templateRoot, p)
	if !strings.HasSuffix(p, ".html") {
		log.Println("serving raw file", p)
		http.ServeFile(resp, req, p)
		return
	}
	log.Println("serving template", p)
	t, err := template.ParseFiles(p)
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

