package lang

import (
	"bytes"
	pgs "github.com/lyft/protoc-gen-star"
	"strings"
	"text/template"
)

const tpl = `
include {{ .ModuleName }}

message = {{ .ClassName }}.new({{ range .Fields }}
  {{ .Name.LowerSnakeCase.String }}: 'abcdef',
{{- end }}
)


`

type RubyTpl struct {
	ModuleName string
	ClassName string
	Fields []pgs.Field
}

func ToRuby(msg pgs.Message) string {
	moduleName := msg.File().Descriptor().GetOptions().GetRubyPackage()
	if moduleName == "" {
		moduleName =  msg.File().Descriptor().GetPackage()
	}

	moduleName = strings.Replace(moduleName, ".", "::", -1)
	className := msg.Name().UpperCamelCase().String()

	ctx := RubyTpl{
		ModuleName: moduleName,
		ClassName: className,
		Fields: msg.NonOneOfFields(),
	}

	buf := bytes.NewBufferString("")
	out := template.Must(template.New("ruby").Parse(tpl))
	out.Execute(buf, ctx)

	return buf.String()
}