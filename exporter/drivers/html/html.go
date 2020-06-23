package html

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/cs3org/cato/exporter"
	"github.com/cs3org/cato/exporter/drivers/registry"
	"github.com/cs3org/cato/resources"
	"github.com/mitchellh/mapstructure"
)

const (
	htmlFile = "index.html"

	configDefaultTemplate = "  <li><b>{{ .Config.FieldName}}</b> - {{ .Config.DataType}}</li>\n" +
		"  <ul>\n" +
		"    <li>{{ .Config.Description}} {{ .ReferenceURL}}</li>\n" +
		"    <li>Default: {{ .EscapedDefaultValue}}</li>\n" +
		"  </ul>"
)

func init() {
	registry.Register("html", New)
}

type mgr struct {
	c *config
}

type config struct {
	DocPaths      map[string]string
	ReferenceBase string
}

type templateParameters struct {
	Config              *resources.FieldInfo
	EscapedDefaultValue string
	ReferenceURL        string
}

func parseConfig(m map[string]interface{}) (*config, error) {
	c := &config{}
	if err := mapstructure.Decode(m, c); err != nil {
		return nil, err
	}
	return c, nil
}

func New(m map[string]interface{}) (exporter.ConfigExporter, error) {
	conf, err := parseConfig(m)
	if err != nil {
		return nil, fmt.Errorf("error parsing conf: %w", err)
	}

	mgr := &mgr{
		c: conf,
	}
	return mgr, nil
}

func (m mgr) ExportConfigs(configs map[string][]*resources.FieldInfo, filePath, rootPath string) error {

	td, err := template.New("htmlDefault").Parse(configDefaultTemplate)
	if err != nil {
		return err
	}

	docFileSuffix, err := filepath.Rel(rootPath, path.Dir(filePath))
	if err != nil {
		return err
	}

	var match string
	for k := range m.c.DocPaths {
		if strings.HasPrefix(docFileSuffix, k) && len(k) > len(match) {
			match = k
		}
	}

	configName, err := filepath.Rel(match, docFileSuffix)
	if err != nil {
		return err
	}

	docsRoot := path.Join(rootPath, m.c.DocPaths[match])
	mdDir := path.Join(docsRoot, configName)
	err = os.MkdirAll(mdDir, 0700)
	if err != nil {
		return err
	}

	lines := []string{}

	for s, fields := range configs {
		lines = append(lines, fmt.Sprintf("\n<h2>struct: %s</h2>", s))
		lines = append(lines, "<ul>")

		for _, f := range fields {
			var escapedDefaultValue string
			var isPointer bool
			if strings.HasPrefix(f.DefaultValue, "url:") {
				decodedVal := strings.TrimPrefix(f.DefaultValue, "url:")
				escapedDefaultValue = fmt.Sprintf(`<a href="%s">%s</a>`, decodedVal, decodedVal)
				f.DefaultValue = decodedVal
				isPointer = true
			} else {
				escapedDefaultValue = f.DefaultValue
			}

			var refURL string
			if m.c.ReferenceBase != "" {
				reference, err := filepath.Rel(rootPath, filePath)
				if err != nil {
					return err
				}
				refURL = fmt.Sprintf(`<a href="%s/%s#L%d">[Ref]</a>`, m.c.ReferenceBase, reference, f.LineNumber)
			}

			params := templateParameters{
				Config:              f,
				EscapedDefaultValue: escapedDefaultValue,
				ReferenceURL:        refURL,
			}

			b := bytes.Buffer{}
			if isPointer {
				err = td.Execute(&b, params)
				if err != nil {
					return err
				}
			} else {
				err = td.Execute(&b, params)
				if err != nil {
					return err
				}
			}
			lines = append(lines, b.String())
		}
		lines = append(lines, "</ul>")
	}

	docFile := path.Join(mdDir, htmlFile)
	fo, err := os.Create(docFile)
	if err != nil {
		return err
	}
	defer fo.Close()
	w := bufio.NewWriter(fo)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	return w.Flush()
}
