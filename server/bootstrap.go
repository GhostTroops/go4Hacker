// +build bootstrap

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

const translateTemplate = `// GENERATED BY THE COMMAND DO NOT EDIT
// This file was generated by bootstrap.go at
// {{ .Date }}

package {{ .Package }}

import (
	"encoding/json"
	"bytes"
	"encoding/base64"
)

var (
	_tsdata = "{{ .Data }}"
	_ts  map[string]TranslateItem
	_tsl TranslateItem
)

type TranslateItem map[string]string

func init() {
	d := json.NewDecoder(base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(_tsdata)))
	d.Decode(&_ts)	
	switchTranslate("en-US")
}

func switchTranslate(lang string) bool {
	if l, ok := _ts[lang]; ok {
		_tsl = l
		return true
	}
	return false
}

func translate(id string) string {
	if _tsl == nil {
		return id
	}
	if r, ok := _tsl[id]; ok {
		return r
	}
	return id
}

func translateByLang(lang, id string) string {
	if tsl, ok := _ts[lang]; ok {
		if r, exist := tsl[id]; exist {
			return r
		}
	}
	return id
}
`

type TranslateItem map[string]string

type translateNodeVisitor struct {
	n map[string]bool
}

func (p *translateNodeVisitor) Visit(n ast.Node) (w ast.Visitor) {
	if x, ok := n.(*ast.CallExpr); ok && len(x.Args) == 1 {
		if f, ok := x.Fun.(*ast.Ident); ok && f.Name == "T" {
			if a, ok := x.Args[0].(*ast.BasicLit); ok && a.Kind == token.STRING {
				str, _ := strconv.Unquote(a.Value)
				p.n[str] = true
			}
		}
	}
	return p
}

// webui.go -> ts.yml
func updateTranslate(in, out string) error {
	// go ast -> map[string]string
	fset := token.NewFileSet()
	txt, err := ioutil.ReadFile(in)
	if err != nil {
		log.Println("read", err)
		return err
	}

	fs, err := parser.ParseFile(fset, in, txt, parser.AllErrors)
	if err != nil {
		log.Println("parsefile", err)
		return err
	}
	var old map[string]TranslateItem
	{
		oldTxt, err := ioutil.ReadFile(out)
		if err == nil && len(oldTxt) > 0 {
			yaml.Unmarshal(oldTxt, &old)
		} else {
			old = make(map[string]TranslateItem)
		}
	}

	var visitor translateNodeVisitor
	visitor.n = make(map[string]bool)
	ast.Walk(&visitor, fs)

	// add new found
	for k, _ := range visitor.n {
		if _, exist := old[k]; !exist {
			newItem := make(TranslateItem)
			newItem["en-US"] = k
			old[k] = newItem
		}
	}
	// delete not exist
	for k, _ := range old {
		if !visitor.n[k] {
			delete(old, k)
		}
	}
	txt, _ = yaml.Marshal(old)
	return ioutil.WriteFile(out, txt, 0644)
}

// ts.yml -> translate.go
func translate(in, out string) error {
	// go ast -> map[string]string
	txt, err := ioutil.ReadFile(in)
	if err != nil {
		return err
	}
	var data map[string]TranslateItem
	err = yaml.Unmarshal(txt, &data)
	if err != nil {
		return err
	}

	// transform from [id-origin]{[lang]:[id-translate]} to [lang]{[id-origin]:[id-translate]}
	var _ts = make(map[string]TranslateItem)
	for id, ti := range data {
		for lang, idts := range ti {
			if tsl, ok := _ts[lang]; ok {
				tsl[id] = idts
			} else {
				_ts[lang] = make(TranslateItem)
			}
		}
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()

	var arg struct {
		Package string
		Data    template.HTML
		Date    template.HTML
	}

	txt, _ = json.Marshal(_ts)
	arg.Package = "server"
	arg.Data = template.HTML(base64.StdEncoding.EncodeToString(txt))
	arg.Date = template.HTML(time.Now().Format(time.RFC3339))

	tmpl, _ := template.New("translate").Parse(translateTemplate)
	tmpl.ExecuteTemplate(f, "translate", &arg)
	return nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Two params are required, update-ts/translate", os.Args)
		return
	}
	cmd := os.Args[1]
	var err error
	switch cmd {
	case "generate":
		err = updateTranslate("webui.go", "ts.yml")
	case "translate":
		err = translate("ts.yml", "translate.go")
	}
	if err != nil {
		log.Println(err)
	}
}
