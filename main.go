package main

import (
	pgs "github.com/lyft/protoc-gen-star"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"sort"
	"strings"
	"text/template"
)

const tpl = `
# {{ packageName .ProtoName }}

{{ range .Files }}
{{ range .AllMessages }}
## {{ .Name }}

~~~protobuf
{{ source .File.InputPath }} 
~~~

{{ .SourceCodeInfo.LeadingComments }}

### Dependencies

{{ range .Imports }}
- {{ packageName .Package.ProtoName }}
{{ end }}

| Parameter | Type |  Label | Comments |
| --------- | ---- | ----- | ----------- |
{{- range .Fields -}}
| {{ .Name }} | {{ messageType .Type.ProtoType }} | {{ messageLabel .Type.ProtoLabel }} | {{ .SourceCodeInfo.TrailingComments }} |
{{ end }}
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
		"packageName": func(name pgs.Name) string {
			parts := name.Split()
			return strings.Join(parts[len(parts) - 2:], ".")
		},
		"messageType": func(protoType pgs.ProtoType) string {
			switch protoType {
			case pgs.DoubleT:
				return "double"
			case pgs.FloatT:
				return "float"
			case pgs.Int32T:
				return "int (32bit)"
			case pgs.Int64T:
				return "int (64bit)"
			case pgs.UInt32T:
				return "unsigned int (32bit)"
			case pgs.UInt64T:
				return "unsigned int (64 bit)"
			case pgs.BoolT:
				return "boolean"
			case pgs.BytesT:
				return "bytes"
			case pgs.EnumT:
				return "enum"
			case pgs.MessageT:
				return "message"
			case pgs.StringT:
				return "string"
			default:
				return ""
			}
		},
		"messageLabel": func(protoLabel pgs.ProtoLabel) string {
			switch protoLabel {
			case pgs.Optional:
				return "optional"
			case pgs.Required:
				return "required"
			case pgs.Repeated:
				return "repeated"
			default:
				return ""
			}
		},
	}

	m.tpl = template.Must(template.New("slate").Funcs(funcs).Parse(tpl))
}

func (m *slateModule) Name() string {
	return "slate"
}

func (m *slateModule) Generate(pkg pgs.Package, hasIndexFile bool) {
	var out string

	if hasIndexFile {
		out = "_" + pkg.ProtoName().LowerSnakeCase().String() + "_pb.md"
	} else {
		out = pkg.ProtoName().LowerSnakeCase().String() + "_pb.md"
	}

	m.AddGeneratorTemplateFile(out, m.tpl, pkg)

}

func (m *slateModule) Execute(_ map[string]pgs.File, pkgs map[string]pgs.Package) []pgs.Artifact {
	indexFile := m.Parameters().Str("index_path")
	hasIndexFile := indexFile != ""

	for _, pkg := range pkgs {
		if strings.HasPrefix(pkg.ProtoName().String(), "google") {
			continue
		}
		m.Generate(pkg, hasIndexFile)
	}

	// todo: this only works with buf's 'strategy: all' setting (or a default protoc invocation)
	// if using buf with the default 'strategy: directory', the plugin will be executed multiple
	// times and will overwrite the file. Therefore, the setting is hidden behind an option.
	if hasIndexFile {
		var includes []string
		languages := strings.Split(m.Parameters().Str("languages"), ";")

		for _, pkg := range pkgs {
			if strings.HasPrefix(pkg.ProtoName().String(), "google") {
				continue
			}
			path := "_" + pkg.ProtoName().LowerSnakeCase().String() + "_pb.md"
			includes = append(includes, path)
			sort.Strings(includes)
		}

		includesYaml := struct {
			Includes      []string
			LanguageTabs  []string `yaml:"language_tabs"`
			Search        bool
			CodeClipboard bool `yaml:"code_clipboard"`
		}{
			Includes:      includes,
			LanguageTabs:  languages,
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
