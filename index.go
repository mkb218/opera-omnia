package main

import "net/http"
import "html/template"
//import "flag"
import "os"
import "path"

func init() {
	http.HandleFunc("/", IndexHandler)
}

func IndexHandler(resp http.ResponseWriter, req *http.Request) {
	// get template
	logMessage(LOG_INFO, "serving template", "index.html")
	t, err := template.ParseFiles(path.Join(templateRoot, "index.html"))
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

