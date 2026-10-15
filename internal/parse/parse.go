package parse

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"devkit-deps/internal/diag"
	"devkit-deps/internal/lang"
)

type Kind string

const (
	KindAbsent       Kind = "absent"
	KindDependencies Kind = "dependencies"
)

type Declaration struct {
	Kind  Kind
	Specs []string
	Line  int
}

var luaLong = regexp.MustCompile(`--\[=*\[`)
var redHeader = regexp.MustCompile(`(?i)^Red\s*\[.*]\s*$`)

func Parse(source string, id lang.ID) (Declaration, *diag.Diagnostic) {
	if strings.HasPrefix(source, "\uFEFF") || bytes.HasPrefix([]byte(source), []byte{0xEF, 0xBB, 0xBF}) {
		return Declaration{}, &diag.Diagnostic{
			Code: "declaration.bom", Message: "UTF-8 BOM is not accepted",
			Language: string(id), Phase: diag.PhaseParse, Line: 1, Exit: 2,
		}
	}
	prefix, err := lang.CommentPrefix(id)
	if err != nil {
		if d, ok := err.(*diag.Diagnostic); ok {
			return Declaration{}, d
		}
		return Declaration{}, &diag.Diagnostic{Code: "language.unknown", Message: err.Error(), Language: string(id), Phase: diag.PhaseParse, Exit: 2}
	}
	declRE := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(prefix) + `\s*devkit-deps:\s*(.*?)\s*$`)
	lines := strings.Split(source, "\n")
	var payloads []string
	var lineNo int
	for i, line := range lines {
		raw := strings.TrimRight(line, "\r")
		text := strings.TrimSpace(raw)
		switch {
		case text == "":
			continue
		case i == 0 && strings.HasPrefix(text, "#!"):
			continue
		case id == lang.PHP && text == "<?php":
			continue
		case id == lang.Red && redHeader.MatchString(text):
			continue
		case id == lang.Lua && luaLong.MatchString(text):
			goto scanned
		case strings.HasPrefix(text, prefix):
			if m := declRE.FindStringSubmatch(raw); m != nil {
				payloads = append(payloads, m[1])
				if lineNo == 0 {
					lineNo = i + 1
				}
			}
		default:
			goto scanned
		}
	}
scanned:
	if len(payloads) == 0 {
		return Declaration{Kind: KindAbsent}, nil
	}
	if len(payloads) != 1 {
		return Declaration{}, &diag.Diagnostic{
			Code: "declaration.duplicate", Message: "use one devkit-deps declaration",
			Language: string(id), Phase: diag.PhaseParse, Line: lineNo, Exit: 2,
		}
	}
	dec := json.NewDecoder(strings.NewReader(payloads[0]))
	var raw []any
	if err := dec.Decode(&raw); err != nil || dec.More() {
		return Declaration{}, &diag.Diagnostic{
			Code: "declaration.json", Message: "devkit-deps must be a JSON array of package strings",
			Language: string(id), Phase: diag.PhaseParse, Line: lineNo, Exit: 2,
		}
	}
	specs := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return Declaration{}, &diag.Diagnostic{
				Code: "declaration.json", Message: "devkit-deps must be a JSON array of package strings",
				Language: string(id), Phase: diag.PhaseParse, Line: lineNo, Exit: 2,
			}
		}
		specs = append(specs, s)
	}
	seen := map[string]struct{}{}
	for i, spec := range specs {
		spec = strings.TrimSpace(spec)
		specs[i] = spec
		if spec == "" || strings.HasPrefix(spec, "-") || hasControl(spec) {
			return Declaration{}, &diag.Diagnostic{
				Code: "package.option", Message: "dependency names must be nonempty package specifications, not command options",
				Language: string(id), Phase: diag.PhaseParse, Line: lineNo, Exit: 2,
			}
		}
		if _, ok := seen[spec]; ok {
			return Declaration{}, &diag.Diagnostic{
				Code: "package.duplicate", Message: "duplicate dependency specifications are not supported",
				Language: string(id), Phase: diag.PhaseParse, Line: lineNo, Exit: 2,
			}
		}
		seen[spec] = struct{}{}
	}
	return Declaration{Kind: KindDependencies, Specs: specs, Line: lineNo}, nil
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
