package wand

import (
	"bytes"
	"html/template"
	"net/http"
)

type PageData struct {
	Link       string
	ErrMsg     string
	StatusCode int
}

func RenderLinkIndex(w http.ResponseWriter, msg string, code int, err error) {
	templates := template.Must(template.ParseGlob("./index.html"))
	pageData := PageData{
		Link:       msg,
		StatusCode: code,
	}
	if err != nil {
		pageData.ErrMsg = err.Error()
	}
	b := new(bytes.Buffer)
	err = templates.ExecuteTemplate(b, "index", pageData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(code)
	w.Write(b.Bytes())
}
