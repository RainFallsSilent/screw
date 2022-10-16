package screw

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
)

func genStructName(k string) string {
	return k + "AutoGen"
}

func genVarName(varName string) string {
	return varName + "Var"
}

// Generate the structure according to the resolved function name and parameters
func genStructBytes(p *ParseFlag) ([]byte, error) {

	var code bytes.Buffer
	var allCode bytes.Buffer

	for k, funcAndArgs := range p.funcAndArgs {
		v := funcAndArgs
		if !v.haveParseFunc {
			continue
		}

		if p.haveImportPath {
			code.WriteString(`

			package main
			import (
				"github.com/fungolang/screw"
			)
			`)
		}

		if !p.haveStruct {
			continue
		}

		code.WriteString(fmt.Sprintf("type %s struct{", genStructName(k)))

		for _, arg := range v.args {
			//The option name is important. If there is no option name, it will not be generated
			if len(arg.optName) == 0 || len(arg.varName) == 0 {
				continue
			}
			//Write field name and type name
			varName := arg.varName
			if varName[0] >= 'a' && varName[0] <= 'z' {
				varName = string(varName[0]-'a'+'A') + varName[1:]
			}

			code.WriteString(fmt.Sprintf("%s %s", varName, arg.typeName))

			var screwTag bytes.Buffer

			//Write Option Name
			if len(arg.optName) > 0 {
				screwTag.WriteString("`screw:\"")
				numMinuses := "-"
				if len(arg.optName) > 1 {
					numMinuses = "--"
				}
				screwTag.WriteString(fmt.Sprintf("%s%s\" ", numMinuses, arg.optName))
			}

			//Write Default
			if len(arg.defVal) > 0 {
				screwTag.WriteString(fmt.Sprintf("default:\"%s\" ", arg.defVal))
			}

			//Write help information
			if len(arg.usage) > 0 {
				screwTag.WriteString(fmt.Sprintf("usage:\"%s\" `\n", arg.usage))
			}

			code.WriteString(screwTag.String())

		}

		code.WriteString("}")
		if p.haveMain {
			varName := strings.ToLower(k)
			code.WriteString(fmt.Sprintf(`
			func main() {
			var %s %s
			screw.Bind(&%s)
			}`, genVarName(varName), genStructName(k), genVarName(varName)))
		}

		fmtCode, err := format.Source(code.Bytes())
		if err != nil {
			return nil, err
		}

		allCode.Write(fmtCode)

		code.Reset()

	}

	return allCode.Bytes(), nil
}
