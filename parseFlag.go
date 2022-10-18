package screw

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

// Name of the parsing function of the flag library, white name
type argsNumAndType struct {
	size     int
	typeName string
}

var funcName = map[string]argsNumAndType{
	//"Func":true, TODO
	"Bool":        {3, "bool"},
	"BoolVar":     {4, "bool"},
	"Duration":    {3, "time.Duration"},
	"DurationVar": {4, "time.Duration"},
	"Float64":     {3, "float64"},
	"Float64Var":  {4, "float64"},
	"Int":         {3, "int"},
	"IntVar":      {4, "int"},
	"Int64":       {3, "int64"},
	"Int64Var":    {4, "int64"},
	"String":      {3, "string"},
	"StringVar":   {4, "string"},
	"Uint":        {3, "uint"},
	"UintVar":     {4, "uint"},
	"Uint64":      {3, "uint64"},
	"Uint64Var":   {4, "uint64"},
}

// Parse flag
type ParseFlag struct {
	astFile        *ast.File
	fileName       string
	funcAndArgs    map[string]funcAndArgs
	haveStruct     bool
	haveImportPath bool
	haveMain       bool
}

// Constructor
func NewParseFlag() *ParseFlag {
	return &ParseFlag{}
}

// Generate struct only
func (p *ParseFlag) OnlyStruct() *ParseFlag {
	p.haveStruct = true
	return p
}

// Generate All
func (p *ParseFlag) All() *ParseFlag {
	p.haveStruct = true
	p.haveImportPath = true
	p.haveMain = true

	return p
}

// Set File Name
func (p *ParseFlag) FromFile(fileName string) *ParseFlag {
	p.fileName = fileName
	return p
}

func (p *ParseFlag) Parse() ([]byte, error) {
	p.funcAndArgs = make(map[string]funcAndArgs)
	if err := p.getFuncCallsToken(); err != nil {
		return nil, err
	}

	return genStructBytes(p)
}

// The address called by each flag library is resolved into the funcAndArgs structure
type funcAndArgs struct {
	args          []flagOpt
	haveParseFunc bool
}

// Save the metadata extracted from the ast
type flagOpt struct {
	varName  string
	optName  string
	defVal   string
	usage    string
	typeName string
}

// It can be determined that it is the function you want, such as flag. String
func isFunc(expr ast.Expr, pkg, fn string) bool {
	f, ok := expr.(*ast.SelectorExpr)
	return ok && isIdent(f.X, pkg) && isIdent(f.Sel, fn)
}

func getPtrArgName(arg ast.Expr) string {
	a, ok := arg.(*ast.UnaryExpr)
	if !ok {
		return ""
	}
	return getIdentName(a.X)
}

// Get Function Name
func getArgName(arg ast.Expr) string {
	a, ok := arg.(*ast.BasicLit)
	if !ok {
		return ""
	}
	return a.Value
}

// Extract function name and formal parameter
func (p *ParseFlag) takeFuncNameAndArgs(expr ast.Expr, args []ast.Expr) (err error) {
	f, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	obj := getIdentName(f.X)
	fn := getIdentName(f.Sel)

	argsNumType, ok := funcName[fn]
	if !ok {
		if fn == "Parse" {
			if v, ok := p.funcAndArgs[obj]; ok {
				v.haveParseFunc = true
				p.funcAndArgs[obj] = v
			}
		}
		return nil
	}

	if _, ok := p.funcAndArgs[obj]; !ok {
		p.funcAndArgs[obj] = funcAndArgs{}
	}

	if argsNumType.size != len(args) {
		return nil
	}

	var opt flagOpt
	strArgs := make([]string, len(args))
	for i := range args {
		arg := ""
		arg2 := ""
		if len(strArgs) == 4 && i == 0 {
			arg = getPtrArgName(args[i])
			goto next
		}

		arg = getArgName(args[i])

	next:
		arg2, err = strconv.Unquote(arg)
		if err != nil {
			arg2 = arg
			//return err
		}

		strArgs[i] = arg2
	}

	if argsNumType.size == 3 {
		opt.varName = obj
		opt.optName = strArgs[0]
		opt.defVal = strArgs[1]
		opt.usage = strArgs[2]

	} else {
		opt.varName = strArgs[0]
		opt.optName = strArgs[1]
		opt.defVal = strArgs[2]
		opt.usage = strArgs[3]
	}

	opt.typeName = argsNumType.typeName
	oldVal := p.funcAndArgs[obj]
	oldVal.args = append(oldVal.args, opt)
	p.funcAndArgs[obj] = oldVal

	return nil
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

func getIdentName(expr ast.Expr) string {
	ident, ok := expr.(*ast.Ident)
	if ok {
		return ident.Name
	}

	return ""
}

// Parsing Flag.NewFlagSet Code
func (p *ParseFlag) parserFlagNewFlagSet(stmt *ast.AssignStmt) {
	if (stmt.Tok == token.ASSIGN || stmt.Tok == token.DEFINE) && len(stmt.Rhs) > 0 {
		if isFunc(stmt.Rhs[0], "flag", "NewFlagSet") && len(stmt.Lhs) > 0 {
			name := getIdentName(stmt.Lhs[0])
			if len(name) == 0 {
				return
			}

			p.funcAndArgs[name] = funcAndArgs{}

		}
	}
}

// Code for parsing function calls
func (p *ParseFlag) findFuncCalls(node ast.Node) bool {
	stmt, ok := node.(*ast.AssignStmt)
	if ok {
		p.parserFlagNewFlagSet(stmt)
		return true
	}

	call, ok := node.(*ast.CallExpr)
	if !ok {
		return true
	}

	err := p.takeFuncNameAndArgs(call.Fun, call.Args)
	if err != nil {
		// debug
		//panic(err.Error())
	}

	return true
}

// Get functions and formal parameters
func (p *ParseFlag) getFuncCallsToken() (err error) {

	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, p.fileName, nil, 0)
	if err != nil {
		return err
	}

	p.funcAndArgs["flag"] = funcAndArgs{}
	p.astFile = f

	ast.Inspect(p.astFile, p.findFuncCalls)
	return nil
}
