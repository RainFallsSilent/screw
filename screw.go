package screw

import (
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"
)

var (
	ErrDuplicateOptions = errors.New("is already in use")
	ErrUnsupported      = errors.New("unsupported command")
	ErrNotFoundName     = errors.New("no command line options found")
	ErrOptionName       = errors.New("illegal option name")
)

var (
	ShowUsageDefault = true
)

const (
	defautlVersion      = "v1.0.1"
	defautlCallbackName = "Parse"
	defaultSubMain      = "SubMain"
)

const (
	optGreedy          = "greedy"
	optOnce            = "once"
	optEnv             = "env"
	optEnvEqual        = "env="
	optSubcommand      = "subcommand"
	optSubcommandEqual = "subcommand="
	optShort           = "short"
	optLong            = "long"
	optCallback        = "callback"
	optCallbackEqual   = "callback="
	optSpace           = " "
)

/*
type SubMain interface {
	SubMain()
}
*/

type unparsedArg struct {
	arg   string
	index int
}

type Screw struct {
	root         *Screw
	shortAndLong map[string]*Option
	checkEnv     map[string]struct{}
	checkArgs    map[string]struct{}
	envAndArgs   []*Option
	args         []string
	unparsedArgs []unparsedArg
	allStruct    map[interface{}]struct{}

	about   string
	version string

	subMain    reflect.Value
	structAddr reflect.Value
	exit       bool
	subcommand map[string]*Subcommand

	isSetSubcommand map[string]struct{}
	procName        string

	currSubcommandFieldName string
	fieldName               string
	w                       io.Writer
}

func (c *Screw) SetVersion(version string) *Screw {
	c.version = version
	return c
}

func (c *Screw) SetAbout(about string) *Screw {
	c.about = about
	return c
}

type Subcommand struct {
	*Screw
	usage string
}

type Option struct {
	pointer      reflect.Value
	fn           reflect.Value
	usage        string
	showDefValue string
	//Indicates the parameter priority. The high 4 bytes store the args sequence,
	//and the low 4 bytes store the command combination sequence (ls ltr).
	//The value of the high 4 bytes of l here is 0
	index    uint64
	envName  string
	argsName string
	//Greedy mode - H a b c equals - H a - H b - H c
	greedy bool
	//If the once flag is set, the command line will report that
	//the repeated option of - debug - debug is invalid for the slice variable
	//It can only be set once. If the once flag is set,
	//an error will be reported if the command line passes the option twice
	once      bool
	cmdSet    bool
	showShort []string
	showLong  []string
}

func (o *Option) onceResetValue() {
	if len(o.showDefValue) > 0 && !o.pointer.IsZero() && !o.cmdSet {
		resetValue(o.pointer)
	}

	o.cmdSet = true
}

func New(args []string) *Screw {
	return &Screw{
		shortAndLong: make(map[string]*Option),
		checkEnv:     make(map[string]struct{}),
		checkArgs:    make(map[string]struct{}),
		//TODO needs to optimize the memory, and only root needs to initialize
		isSetSubcommand: make(map[string]struct{}),
		allStruct:       make(map[interface{}]struct{}),
		args:            args,
		exit:            true,
		w:               os.Stdout,
	}
}

// Check the legitimacy of the option name
func checkOptionName(name string) (byte, bool) {
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			continue
		}
		return c, false
	}
	return 0, true
}

// Set the error behavior. By default, an error will exit the process (true). If it is false, it will not
func (c *Screw) SetExit(exit bool) *Screw {
	c.exit = exit
	return c
}

func (c *Screw) SetOutput(w io.Writer) *Screw {
	c.w = w
	return c
}

// Set Process Name
func (c *Screw) SetProcName(procName string) *Screw {
	c.procName = procName
	return c
}

func (c *Screw) IsSetSubcommand(subcommand string) bool {
	_, ok := c.isSetSubcommand[subcommand]
	return ok
}

func (c *Screw) GetIndex(optName string) uint64 {
	o, ok := c.shortAndLong[optName]
	if ok {
		return o.index
	}
	if _, ok := c.checkArgs[optName]; ok {
		for _, o := range c.envAndArgs {
			if o.argsName == optName {
				return o.index
			}
		}
	}

	return 0
}

func (c *Screw) setOption(name string, option *Option, m map[string]*Option, long bool) error {
	if c, ok := checkOptionName(name); !ok {
		return fmt.Errorf("%w:%s:unsupported characters found(%c)", ErrOptionName, name, c)
	}

	if o, ok := m[name]; ok {
		name = "-" + name
		if long {
			name = "-" + name
		}
		return fmt.Errorf("%s %w, duplicate definition with %s", name, ErrDuplicateOptions, c.showShortAndLong(o))
	}

	m[name] = option
	return nil
}

func setValueAndIndex(val string, option *Option, index int, lowIndex int) error {
	option.onceResetValue()
	option.index = uint64(index) << 31
	option.index |= uint64(lowIndex)
	if option.fn.IsValid() {
		//If a callback is defined, the default form of
		option.fn.Call([]reflect.Value{reflect.ValueOf(val)})
		return nil
	}

	return setBase(val, option.pointer)
}

func errOnce(optionName string) error {
	return fmt.Errorf(`error: The argument '-%s' was provided more than once, but cannot be used multiple times`,
		optionName)
}

func (c *Screw) unknownOptionErrorShort(optionName string, arg string) error {
	m := fmt.Sprintf(`error: Found argument '-%s' which wasn't expected, or isn't valid in this context`,
		optionName)

	m += c.genMaybeHelpMsg(arg)
	return errors.New(m)
}

func (c *Screw) unknownOptionError(optionName string) error {
	m := fmt.Sprintf(`error: Found argument '--%s' which wasn't expected, or isn't valid in this context`,
		optionName)

	m += c.genMaybeHelpMsg(optionName)
	return errors.New(m)
}

func setBoolAndBoolSliceDefval(pointer reflect.Value, value *string) {
	kind := pointer.Kind()
	//if bool type, regardless of false
	if *value == "" {
		if reflect.Bool == kind {
			*value = "true"
			return
		}

		if _, isBoolSlice := pointer.Interface().([]bool); isBoolSlice {
			*value = "true"
		}
	}

	return

}

func (c *Screw) parseEqualValue(arg string) (value string, option *Option, err error) {
	pos := strings.Index(arg, "=")
	if pos == -1 {
		return "", nil, c.unknownOptionError(arg)
	}

	option, _ = c.shortAndLong[arg[:pos]]
	if option == nil {
		return "", nil, c.unknownOptionError(arg)
	}
	value = arg[pos+1:]
	return value, option, nil
}

func checkOnce(arg string, option *Option) error {
	if option.once && !option.pointer.IsZero() {
		return errOnce(arg)
	}
	return nil
}

func (c *Screw) isRegisterOptions(arg string) bool {
	num := 0
	if len(arg) > 0 && arg[0] == '-' {
		num++
	}

	if len(arg) > 1 && arg[1] == '-' {
		num++
	}

	//Handling the case of value with=
	end := len(arg)
	if e := strings.IndexByte(arg, '='); e != -1 {
		end = e
	}

	_, ok := c.shortAndLong[arg[num:end]]
	return ok
}

// Parse long options
func (c *Screw) parseLong(arg string, index *int) (err error) {
	var option *Option
	value := ""
	option, _ = c.shortAndLong[arg]
	if option == nil {
		if value, option, err = c.parseEqualValue(arg); err != nil {
			return err
		}
	}

	if len(arg) == 1 {
		return c.unknownOptionError(arg)
	}

	//Set the default values of bool and bool slice
	setBoolAndBoolSliceDefval(option.pointer, &value)

	if len(value) > 0 {
		if err := checkOnce(arg, option); err != nil {
			return err
		}
		return setValueAndIndex(value, option, *index, 0)
	}

	//If it is a long option
	if *index+1 >= len(c.args) {
		return nil
	}

	for {

		(*index)++
		if *index >= len(c.args) {
			return nil
		}

		value = c.args[*index]

		if c.findFallbackOpt(value, index) {
			return nil
		}

		if err := checkOnce(arg, option); err != nil {
			return err
		}

		if err := setValueAndIndex(value, option, *index, 0); err != nil {
			return err
		}

		/*
			if option.pointer.Kind() != reflect.Slice && !option.greedy {
				return nil
			}
		*/

		if !option.greedy {
			return nil
		}
	}

	return nil
}

// Setting environment variables and parameters
func (o *Option) setEnvAndArgs(c *Screw) (err error) {
	if len(o.envName) > 0 {
		if v, ok := os.LookupEnv(o.envName); ok {
			if o.pointer.Kind() == reflect.Bool {
				if v != "false" {
					v = "true"
				}
			}

			return setValueAndIndex(v, o, 0, 0)
		}
	}

	if len(o.argsName) > 0 {
		if len(c.unparsedArgs) == 0 {
			//todo修饰下报错信息
			//return errors.New("unparsedargs == 0")
			return nil
		}

		value := c.unparsedArgs[0]
		switch o.pointer.Kind() {
		case reflect.Slice:
			for o.pointer.Kind() == reflect.Slice {
				setValueAndIndex(value.arg, o, value.index, 0)
				c.unparsedArgs = c.unparsedArgs[1:]
				if len(c.unparsedArgs) == 0 {
					break
				}

				value = c.unparsedArgs[0]
			}
		default:
			if err := setValueAndIndex(value.arg, o, value.index, 0); err != nil {
				return err
			}
			if len(c.unparsedArgs) > 0 {
				c.unparsedArgs = c.unparsedArgs[1:]
			}
		}

	}
	return nil
}

func (c *Screw) parseShort(arg string, index *int) error {
	var (
		option     *Option
		shortIndex int
	)

	var a rune
	find := false
	//Examples of parameter types that can be resolved
	//- d - d is bool type
	//- vvv is [] bool type
	//- d=false - d is bool false is value
	//- f file - f is a string type, and file is a value
	for shortIndex, a = range arg {
		//Only ascii is supported
		if a >= utf8.RuneSelf {
			return errors.New("Illegal character set")
		}

		optionName := string(byte(a))
		option, _ = c.shortAndLong[optionName]
		if option == nil {
			//没有注册过的选项直接报错
			return c.unknownOptionErrorShort(optionName, arg)
		}

		find = true
		findEqual := false //Whether equal sign is found
		value := arg
		_, isBoolSlice := option.pointer.Interface().([]bool)
		_, isBool := option.pointer.Interface().(bool)
		if !(isBoolSlice || isBool) {
			shortIndex++
		}

		if len(value[shortIndex:]) > 0 && len(value[shortIndex+1:]) > 0 {
			if value[shortIndex:][0] == '=' {
				findEqual = true
				shortIndex++
			}

			if value[shortIndex+1:][0] == '=' {
				findEqual = true
				shortIndex += 2
			}
		}

	getchar:
		for value := arg; ; {
			//If there is no value, the next args parameter should be taken
			if len(value[shortIndex:]) > 0 {
				val := value[shortIndex:]
				if isBoolSlice || isBool {
					val = "true"
				}

				if findEqual {
					val = string(value[shortIndex:])
				}

				if err := checkOnce(value[shortIndex:], option); err != nil {
					return err
				}

				if err := setValueAndIndex(val, option, *index, shortIndex); err != nil {
					return err
				}

				if findEqual {
					return nil
				}

				if isBoolSlice || isBool { //For example, in the case of - vvv
					break getchar
				}

				/*
					// NonGreedy mode, parsing and setting slice variables will eat more variables required by args parameters
					if option.pointer.Kind() != reflect.Slice && !option.greedy {
						return nil
					}
				*/
				if !option.greedy {
					return nil
				}
			}

			shortIndex = 0

			if *index+1 >= len(c.args) {
				return nil
			}
			(*index)++

			value = c.args[*index]

			if c.findFallbackOpt(value, index) {
				return nil
			}

		}

	}

	if find {
		return nil
	}

	return c.unknownOptionErrorShort(arg, arg)
}

func (c *Screw) findFallbackOpt(value string, index *int) bool {

	//If greedy mode is turned on, it will not end until - or the last character is encountered
	if strings.HasPrefix(value, "-") {
		//If this is a command line option instead of a negative number, the option will be rolled back directly
		if c.isRegisterOptions(value) {
			(*index)-- //Fallback this option
			return true
		}
	}

	return false
}

func (c *Screw) getOptionAndSet(arg string, index *int, numMinuses int) error {
	if arg == "h" || arg == "help" {
		if _, ok := c.shortAndLong[arg]; !ok {
			c.Usage()
			return nil
		}
	}

	if arg == "v" || arg == "version" {
		if _, ok := c.shortAndLong[arg]; !ok {
			c.showVersion()
			return nil
		}
	}
	//Take out the option object
	switch numMinuses {
	case 2: //Long Options
		return c.parseLong(arg, index)
	case 1: //Short options
		return c.parseShort(arg, index)
	}

	return nil
}

// ENV_NAME=
// ENV_NAME
func (o *Option) genShowEnvNameValue() (env string) {
	if len(o.envName) > 0 {
		envValue := os.Getenv(o.envName)
		env = o.envName
		if len(envValue) > 0 {
			env = env + "=" + envValue
		}
	}
	return
}

func (c *Screw) showShortAndLong(v *Option) string {
	var oneArgs []string

	for _, v := range v.showShort {
		oneArgs = append(oneArgs, "-"+v)
	}

	for _, v := range v.showLong {
		oneArgs = append(oneArgs, "--"+v)
	}
	return strings.Join(oneArgs, ",")
}

func (c *Screw) genHelpMessage(h *Help) {

	//ShortAndLong Multiple keys point to one option, which requires used map de duplication
	used := make(map[*Option]struct{}, len(c.shortAndLong))

	if c.shortAndLong["h"] == nil && c.shortAndLong["help"] == nil {
		c.shortAndLong["h"] = &Option{usage: "print the help information", showShort: []string{"h"}, showLong: []string{"help"}}
	}

	if c.shortAndLong["v"] == nil && c.shortAndLong["version"] == nil {
		c.shortAndLong["v"] = &Option{usage: "print version information", showShort: []string{"v"}, showLong: []string{"version"}}
	}

	saveHelp := func(options map[string]*Option) {
		for _, v := range options {
			if _, ok := used[v]; ok {
				continue
			}

			used[v] = struct{}{}

			env := v.genShowEnvNameValue()

			opt := c.showShortAndLong(v)

			if h.MaxNameLen < len(opt) {
				h.MaxNameLen = len(opt)
			}

			switch v.pointer.Kind() {
			case reflect.Bool:
				h.Flags = append(h.Flags, showOption{Opt: opt, Usage: v.usage, Env: env, Default: v.showDefValue})
			default:
				h.Options = append(h.Options, showOption{Opt: opt, Usage: v.usage, Env: env, Default: v.showDefValue})
			}
		}
	}

	saveHelp(c.shortAndLong)

	for _, v := range c.envAndArgs {
		opt := v.argsName
		if len(opt) == 0 && len(v.envName) > 0 {
			opt = v.envName
		}

		//Args parameter
		oldOpt := opt
		if len(opt) > 0 {
			opt = "<" + opt + ">"
		}
		if h.MaxNameLen < len(opt) {
			h.MaxNameLen = len(opt)
		}

		env := v.genShowEnvNameValue()
		if len(env) > 0 {
			h.Envs = append(h.Envs, showOption{Opt: oldOpt, Usage: v.usage, Env: env})
			continue
		}

		h.Args = append(h.Args, showOption{Opt: opt, Usage: v.usage, Env: env})
	}

	//Sub command
	for opt, v := range c.subcommand {
		if h.MaxNameLen < len(opt) {
			h.MaxNameLen = len(opt)
		}
		h.Subcommand = append(h.Subcommand, showOption{Opt: opt, Usage: v.usage})
	}

	h.ProcessName = c.procName
	h.Version = c.version
	h.About = c.about
	h.ShowUsageDefault = ShowUsageDefault
}

// Display version information
func (c *Screw) showVersion() {
	fmt.Fprintln(c.w, c.version)
	if c.exit {
		os.Exit(0)
	}
}

func (c *Screw) Usage() {
	c.printHelpMessage()
	if c.exit {
		os.Exit(0)
	}
}

func (c *Screw) printHelpMessage() {
	h := Help{}

	c.genHelpMessage(&h)

	err := h.output(c.w)
	if err != nil {
		panic(err)
	}

}

func (c *Screw) getRoot() (root *Screw) {
	root = c
	if c.root != nil {
		root = c.root
	}
	return root
}

func (c *Screw) parseSubcommandTag(screw string, v reflect.Value, usage string, fieldName string) (newScrew *Screw, haveSubcommand bool) {
	options := strings.Split(screw, ";")
	for _, opt := range options {
		var name string
		switch {
		case strings.HasPrefix(opt, optSubcommandEqual):
			name = opt[len(optSubcommandEqual):]
		case opt == optSubcommand:
			name = strings.ToLower(fieldName)
		}
		if name != "" {
			if c.subcommand == nil {
				c.subcommand = make(map[string]*Subcommand, 3)
			}

			newScrew := New(nil)
			//newScrew.exit = c.exit //Inherit exit attribute
			newScrew.SetProcName(name)
			newScrew.root = c.getRoot()
			c.subcommand[name] = &Subcommand{Screw: newScrew, usage: usage}
			newScrew.fieldName = fieldName

			newScrew.subMain = v.Addr().MethodByName(defaultSubMain)
			return newScrew, true
		}
	}

	return nil, false
}

func (c *Screw) parseTagAndSetOption(screw string, usage string, def string, fieldName string, v reflect.Value) (err error) {
	options := strings.Split(screw, ";")

	option := &Option{usage: usage, pointer: v, showDefValue: def}

	const (
		isShort = 1 << iota
		isLong
		isEnv
		isArgs
	)

	flags := 0
	for _, opt := range options {
		opt = strings.TrimLeft(opt, optSpace)
		if len(opt) == 0 {
			continue //Skip nil values
		}
		name := ""
		//TODO checks the length of name
		switch {
		case strings.HasPrefix(opt, optCallback):
			funcName := defautlCallbackName
			if strings.HasPrefix(opt, optCallbackEqual) {
				funcName = opt[len(optCallbackEqual):]
			}
			option.fn = c.structAddr.MethodByName(funcName)
			//Check the parameter length of callback
			if option.fn.Type().NumIn() != 1 {
				panic(fmt.Sprintf("Required function parameters->%s(val string)", funcName))
			}

		//Registrar Option -- name
		case strings.HasPrefix(opt, "--"):
			name = opt[2:]
			fallthrough
		case strings.HasPrefix(opt, optLong):
			if !strings.HasPrefix(opt, "--") {
				if name, err = gnuOptionName(fieldName); err != nil {
					return err
				}
			}

			if err := c.setOption(name, option, c.shortAndLong, true); err != nil {
				return err
			}
			option.showLong = append(option.showLong, name)
			flags |= isShort
			//Register short options
		case strings.HasPrefix(opt, "-"):
			name = opt[1:]
			fallthrough
		case strings.HasPrefix(opt, optShort):
			if !strings.HasPrefix(opt, "-") {
				if name, err = gnuOptionName(fieldName); err != nil {
					return err
				}
				name = string(name[0])
			}

			if err := c.setOption(name, option, c.shortAndLong, false); err != nil {
				return err
			}
			option.showShort = append(option.showShort, name)
			flags |= isLong
		case strings.HasPrefix(opt, optGreedy):
			option.greedy = true
		case strings.HasPrefix(opt, optOnce):
			option.once = true
		case opt == optEnv:
			if name, err = envOptionName(fieldName); err != nil {
				return err
			}
			fallthrough
		case strings.HasPrefix(opt, optEnvEqual):
			flags |= isEnv
			if strings.HasPrefix(opt, optEnvEqual) {
				name = opt[4:]
			}

			option.envName = name
			if _, ok := c.checkEnv[option.envName]; ok {
				return fmt.Errorf("%s: env=%s", ErrDuplicateOptions, option.envName)
			}
			c.envAndArgs = append(c.envAndArgs, option)
			c.checkEnv[option.envName] = struct{}{}
		case strings.HasPrefix(opt, "args="):
			//Args is mutually exclusive with long and short options
			if flags&isShort > 0 || flags&isLong > 0 {
				continue
			}

			flags |= isArgs
			option.argsName = opt[5:]
			if _, ok := c.checkArgs[option.argsName]; ok {
				return fmt.Errorf("%s: args=%s", ErrDuplicateOptions, option.argsName)
			}

			c.checkArgs[option.argsName] = struct{}{}
			c.envAndArgs = append(c.envAndArgs, option)

		default:
			return fmt.Errorf("%s:(%s) screw(%s)", ErrUnsupported, opt, screw)
		}

		if strings.HasPrefix(opt, "-") && len(name) == 0 {
			return fmt.Errorf("Illegal command line option:%s", opt)
		}

	}

	if flags&isShort == 0 && flags&isLong == 0 && flags&isEnv == 0 && flags&isArgs == 0 {
		return fmt.Errorf("%s:%s", ErrNotFoundName, screw)
	}

	return nil
}

func (c *Screw) registerCore(v reflect.Value, sf reflect.StructField) error {
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	screw := Tag(sf.Tag).Get("screw")
	usage := Tag(sf.Tag).Get("usage")

	//If it is a subcommand
	if v.Kind() == reflect.Struct {
		if len(screw) != 0 {
			if newScrew, b := c.parseSubcommandTag(screw, v, usage, sf.Name); b {
				c = newScrew
			}
		}
	}

	if v.Kind() != reflect.Struct {

		def := Tag(sf.Tag).Get("default")
		def = strings.TrimSpace(def)
		if len(def) > 0 {
			if err := setDefaultValue(def, v); err != nil {
				return err
			}
		}

		if len(screw) == 0 && len(usage) == 0 {
			return nil
		}

		//If it is a field storing version
		if strings.HasPrefix(screw, "version=") {
			c.version = screw[8:]
			return nil
		}

		//If it is a field storing about
		if strings.HasPrefix(screw, "about=") {
			c.about = screw[6:]
			return nil
		}

		if len(usage) > 0 {
			if len(screw) == 0 {
				lowerScrew := strings.ToLower(sf.Name)
				screw = "-" + string(lowerScrew[0])
				if len(lowerScrew) > 1 {
					screw = screw + ";--" + lowerScrew
				}
			}
		}

		return c.parseTagAndSetOption(screw, usage, def, sf.Name, v)
	}

	typ := v.Type()
	c.structAddr = v.Addr()
	for i := 0; i < v.NumField(); i++ {
		sf := typ.Field(i)

		if sf.PkgPath != "" && !sf.Anonymous {
			continue
		}

		//fmt.Printf("my.index(%d)(1.%s)-->(2.%s)\n", i, Tag(sf.Tag).Get("screw"), Tag(sf.Tag).Get("usage"))
		//fmt.Printf("stdlib.index(%d)(1.%s)-->(2.%s)\n", i, sf.Tag.Get("screw"), sf.Tag.Get("usage"))
		if err := c.registerCore(v.Field(i), sf); err != nil {
			return err
		}
	}

	return nil
}

var emptyField = reflect.StructField{}

func (c *Screw) register(x interface{}) error {
	if x == nil {
		return ErrUnsupportedType
	}

	v := reflect.ValueOf(x)

	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("%s:got(%T)", ErrNotPointerType, v.Type())
	}

	//If v is not a pointer, the v.IsNil() function call will crash,
	//so the pointer should be put in front to judge
	if v.IsNil() {
		return ErrUnsupportedType
	}

	c.allStruct[x] = struct{}{}
	return c.registerCore(v, emptyField)
}

func (c *Screw) parseOneOption(index *int) error {

	arg := c.args[*index]

	if len(arg) == 0 {
		return errors.New("fail option")
	}

	if arg[0] != '-' {
		if len(c.subcommand) > 0 {
			newScrew, ok := c.subcommand[arg]
			//The subcommands and args do not start with a - sign. If env or args are not set,
			//they are regarded as unregistered subcommands, and an error is directly reported
			if !ok && len(c.envAndArgs) == 0 {
				return fmt.Errorf("Unknown subcommand:%s", arg)
			}

			c.getRoot().isSetSubcommand[arg] = struct{}{}
			if c.root == nil {
				c.currSubcommandFieldName = newScrew.fieldName
			}

			newScrew.args = c.args[*index+1:]
			c.args = c.args[0:0]
			err := newScrew.bindStruct()
			if err != nil {
				return err
			}
			if newScrew.subMain.IsValid() {
				newScrew.subMain.Call([]reflect.Value{})
			}
		}
		c.unparsedArgs = append(c.unparsedArgs, unparsedArg{arg: arg, index: *index})
		return nil
	}

	//Arg must be a string starting with a minus sign
	numMinuses := 1

	if arg == "-" {
		c.unparsedArgs = append(c.unparsedArgs, unparsedArg{arg: arg, index: *index})
		return nil
	}

	if arg[1] == '-' {
		numMinuses++
	}

	a := arg[numMinuses:]
	return c.getOptionAndSet(a, index, numMinuses)
}

// Set environment variables
func (c *Screw) bindEnvAndArgs() error {
	for _, o := range c.envAndArgs {
		if err := o.setEnvAndArgs(c); err != nil {
			return err
		}
	}

	return nil
}

// Bind structure
func (c *Screw) bindStruct() error {

	for i := 0; i < len(c.args); i++ {

		if err := c.parseOneOption(&i); err != nil {
			return err
		}

	}

	return c.bindEnvAndArgs()
}

func (c *Screw) Bind(x interface{}) (err error) {
	//If c.version is empty, give a default value
	if c.version == "" {
		c.version = defautlVersion
	}

	defer func() {
		if err != nil {
			fmt.Fprintln(c.w, err)
			fmt.Fprintln(c.w, "For more information try --help")
			if c.exit {
				os.Exit(1)
			}
		}
	}()

	if err = c.register(x); err != nil {
		return err
	}

	if err = c.bindStruct(); err != nil {
		return err
	}

	if len(c.currSubcommandFieldName) > 0 {
		v := reflect.ValueOf(x)
		v = v.Elem() // can only be a pointer, which has been judged in c.register
		v = v.FieldByName(c.currSubcommandFieldName)
		//Only the set subcommands need data verification
		//Delete the root structure here
		delete(c.allStruct, x)
		x = v.Addr().Interface()
	}

	c.allStruct[x] = struct{}{}

	for x := range c.allStruct {
		err = valid.ValidateStruct(x)
		if err != nil {
			errs := err.(validator.ValidationErrors)

			for _, e := range errs {
				// can translate each error one at a time.
				return errors.New(e.Translate(valid.trans))
			}

		}
	}
	return err
}

// MustBind is similar to Bind function, and the error is direct panic
func (c *Screw) MustBind(x interface{}) {
	if err := c.Bind(x); err != nil {
		panic(err.Error())
	}
}

// Only register the structure information and do not parse
func (c *Screw) Register(x interface{}) error {
	return c.register(x)
}

// Print Help
func Usage() {
	CommandLine.Usage()
}

func MustRegister(x interface{}) {
	err := CommandLine.Register(x)
	if err != nil {
		panic(err.Error())
	}
}

// Bind interface, including the following functions
// Structure field registration
// Command line parsing
func Bind(x interface{}) error {
	CommandLine.SetProcName(os.Args[0])
	return CommandLine.Bind(x)
}

func SetVersion(version string) {
	CommandLine.SetVersion(version)
}

func SetAbout(about string) {
	CommandLine.SetAbout(about)
}

// Bind must be a successful version
func MustBind(x interface{}) {
	CommandLine.SetProcName(os.Args[0])
	CommandLine.MustBind(x)
}

func IsSetSubcommand(subcommand string) bool {
	return CommandLine.IsSetSubcommand(subcommand)
}

func GetIndex(optName string) uint64 {
	return CommandLine.GetIndex(optName)
}

var CommandLine = New(os.Args[1:])
