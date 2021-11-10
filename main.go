package main

import (
	pgs "github.com/lyft/protoc-gen-star"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"strings"
	"text/template"
)

const tpl = `
{{- range .AllMessages -}}
# {{ .Name }}

~~~protobuf
{{ source .File.InputPath }} 
~~~

{{ .SourceCodeInfo.LeadingDetachedComments }}

{{ .SourceCodeInfo.LeadingComments }}

**Dependencies**

{{ range .Imports }}
- [{{ .Name }}]({{ .InputPath }})
{{ end }}

| Parameter | Type | Comments |
| --------- | ---- | ----------- |
{{- range .Fields -}}
| {{ .Name }} | {{ .Type.ProtoType }} | {{ .SourceCodeInfo.TrailingComments }} |
{{ end }}
{{ end }}
`

type slateModule struct {
	*pgs.ModuleBase
	tpl *template.Template
}

func (m *slateModule) InitContext(c pgs.BuildContext) {
	m.ModuleBase.InitContext(c)

	funcs := map[string]interface{}{
		"source": func(fp pgs.FilePath) string {
			// todo: don't hard-code schemas path. Only needed because of how buf works
			res, err := os.ReadFile("schemas/" + fp.String())
			if err != nil {
				log.Fatal(err)
			}
			return string(res)
		},
	}

	m.tpl = template.Must(template.New("slate").Funcs(funcs).Parse(tpl))
}

func (m *slateModule) Name() string {
	return "slate"
}

func (m *slateModule) Generate(f pgs.File, hasIndexFile bool) {
	var out string

	if hasIndexFile {
		out = f.InputPath().Dir().String() + "/_" + strings.TrimSuffix(f.InputPath().BaseName(), ".proto") + "_pb.md"
	} else {
		out = strings.TrimSuffix(f.InputPath().String(), ".proto") + "_pb.md"
	}

	m.AddGeneratorTemplateFile(out, m.tpl, f)

}

func (m *slateModule) Execute(files map[string]pgs.File, _ map[string]pgs.Package) []pgs.Artifact {
	indexFile := m.Parameters().Str("index_path")
	hasIndexFile := indexFile != ""

	for _, file := range files {
		m.Generate(file, hasIndexFile)
	}

	// todo: this only works with buf's 'strategy: all' setting (or a default protoc invocation)
	// if using buf with the default 'strategy: directory', the plugin will be executed multiple
	// times and will overwrite the file. Therefore, the setting is hidden behind an option.
	if hasIndexFile {
		var includes []string

		for path, _ := range files {
			includes = append(includes, strings.TrimSuffix(path, ".proto")+"_pb.md")
		}

		includesYaml := struct {
			Includes      []string
			LanguageTabs  []string `yaml:"language_tabs"`
			Search        bool
			CodeClipboard bool `yaml:code_clipboard`
		}{
			Includes:      includes,
			LanguageTabs:  []string{"protobuf"},
			Search:        true,
			CodeClipboard: true,
		}

		yamlStr, err := yaml.Marshal(includesYaml)
		if err != nil {
			log.Fatal(err)
		}

		m.AddCustomFile(indexFile, "---\n"+string(yamlStr)+"---\n", 0777)
	}

	return m.Artifacts()
}

func Slate() *slateModule {
	return &slateModule{ModuleBase: &pgs.ModuleBase{}}
}

func main() {
	pgs.Init(pgs.DebugEnv("DEBUG")).RegisterModule(Slate()).Render()
}
