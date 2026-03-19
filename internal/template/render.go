package template

import (
	"html/template"
	"io/fs"
	"net/http"
)

type Renderer struct {
	fs fs.FS
}

func NewRenderer(templateFS fs.FS) *Renderer {
	return &Renderer{fs: templateFS}
}

func (r *Renderer) Render(w http.ResponseWriter, page string, data interface{}) {
	tmpl, err := template.ParseFS(r.fs, "layouts/base.html", "pages/"+page+".html")
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
	}
}
