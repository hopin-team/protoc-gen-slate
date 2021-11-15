package main

import (
	"github.com/hopin-team/protoc-gen-slate/lang"
	"github.com/iancoleman/strcase"
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

{{ range languageTabs . }}
~~~{{ .Language }}
{{ .CodeExample }}
~~~
{{- end }}

{{ .SourceCodeInfo.LeadingComments }}

{{ if .Imports }}
### Imports

{{ range packageLinks .Imports }}
- [{{ .Name }}]({{ .Url }})
{{ end }}
{{ end }}

### Fields

| Parameter | Type |  Label | Comments |
| --------- | ---- | ----- | ----------- |
{{ range .NonOneOfFields -}}
| {{ .Name }} | {{ messageType .Type.ProtoType }} | {{ messageLabel .Type.ProtoLabel }} | {{ .SourceCodeInfo.TrailingComments }} |
{{ end -}}
{{ range .OneOfs -}}
| {{ .Name }} | {{ oneOfFieldNames .Fields }} | |  |
{{ end }}

{{ end }}
{{ end }}
`

type LanguageTab struct {
	Language    string
	CodeExample string
}

type PackageLink struct {
	Name string
	Url string
}

var languageProcessors = map[string]func(pgs.Message) LanguageTab{
	"ruby": func(msg pgs.Message) LanguageTab {
		return LanguageTab{
			Language:    "ruby",
			CodeExample: lang.ToRuby(msg),
		}

	},
	"javascript": func(msg pgs.Message) LanguageTab {
		return LanguageTab{
			Language:    "javascript",
			CodeExample: "",
		}
	},
	"java": func(msg pgs.Message) LanguageTab {
		return LanguageTab{
			Language:    "java",
			CodeExample: "",
		}
	},
	"python": func(msg pgs.Message) LanguageTab {
		return LanguageTab{
			Language:    "python",
			CodeExample: "",
		}
	},
	"protobuf": func(msg pgs.Message) LanguageTab {
		res, err := os.ReadFile("schemas/" + msg.File().InputPath().String())
		if err != nil {
			log.Fatal(err)
		}

		return LanguageTab{
			Language:    "protobuf",
			CodeExample: string(res),
		}
	},
}

type slateModule struct {
	*pgs.ModuleBase
	tpl *template.Template
}

func (m *slateModule) InitContext(c pgs.BuildContext) {
	m.ModuleBase.InitContext(c)

	funcs := map[string]interface{}{
		"packageName": func(name pgs.Name) string {
			parts := name.Split()
			return strings.Join(parts[len(parts)-2:], ".")
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
		"oneOfFieldNames": func(fields []pgs.Field) string {
			var fieldNames []string
			for _, field := range fields {
				fieldNames = append(fieldNames, field.Name().UpperCamelCase().String())
			}

			return strings.Join(fieldNames, "<br />")
		},
		"packageLinks": func(files []pgs.File) []PackageLink {
			var packageLinks []PackageLink

			for _, file := range files {
				if strings.HasPrefix(file.Package().ProtoName().String(), "google") {
					continue
				}

				pkgParts := file.Package().ProtoName().LowerSnakeCase().Split()
				fileName := strings.TrimSuffix(file.InputPath().BaseName(), ".proto")

				packageLink := PackageLink{
					Name: strcase.ToCamel(fileName),
					Url: "#" + strings.Join(pkgParts[len(pkgParts)-3:], "-") + "-" + strings.ToLower(fileName),
				}
				packageLinks = append(packageLinks, packageLink)
			}

			return packageLinks
		},
		"languageTabs": func(message pgs.Message) []LanguageTab {
			languages := strings.Split(m.Parameters().Str("languages"), ";")
			var langTabs []LanguageTab

			for _, language := range languages {
				langTabs = append(langTabs, languageProcessors[language](message))
			}
			return langTabs
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
