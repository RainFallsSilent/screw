# screw

[![Go](https://github.com/fungolang/screw/workflows/Go/badge.svg)](https://github.com/fungolang/screw/actions)


screw is a command line parser based on struct.

## Feature

* Support for environment variable binding ```env DEBUG=xx ./proc```
* Supports parameter collection ```cat a.txt b.txt```, you can put```a.txt, b.txt```Bulk members are categorized and collected into the structure members you specify
* Support for short options```proc -d``` or long options```proc --debug```
* POSIX style command line support, supporting command combinations ```ls -ltr```is```ls -l -t -r```abbreviated to facilitate the implementation of common POSIX standard commands
* Sub Command(```subcommand```)support to facilitate git-style subcommand```git add ```，a concise way of registering subcommands, as long as the structure is written, 3,4,5 to infinite subcommands are also supported
* Defaults support```default:"1"```, support multiple data types so you don't have to worry about type conversion
* Repeat Command Error
* Strict short option, long option error. Avoid Ambiguity Options
* Validation mode support, no need to write a bunch of```if x!= "" ``` or ```if y!=0```
* command priority can be obtained to set command aliases easily
* Parse flag package code to generate screw code
* Specify`Parse`function to automatically bind data

- [Installation](#Installation)
- [Quick start](#quick-start)
- [example](#example)
	- [base type](#base-type)
		- [int](#int)
		- [float64](#float64)
		- [time.Duration](#duration)
		- [string](#string)
	- [array](#array)
		- [similar to curl command](#similar-to-curl-command)
		- [similar to join command](#similar-to-join-command)
	- [1. How to use required tags](#required-flag)
	- [2. Support environment variables](#support-environment-variables)
		- [2.1 Custom environment variable name](#custom-environment-variable-name)
		- [2.2 Quick writing of environment variables](#quick-writing-of-environment-variables)
	- [3. Set default value](#set-default-value)
	- [4. How to implement git style commands](#subcommand)
		- [4.1 Sub command implementation method 1](#sub-command-implementation-method-1)
		- [4.2 Sub command implementation method 2](#sub-command-implementation-method-2)
	- [5. Get command priority](#get-command-priority)
	- [6. Can only be set once](#can-only-be-set-once)
	- [7. Quick write](#quick-write)
	- [8. Multi structure series](#multi-structure-series)
	- [9. Support callback function parsing](#support-callback-function-parsing)
	- [Advanced features](#Advanced-features)
		- [Parsing flag code to generate screw code](#Parsing-flag-code-to-generate-screw-code)
- [Implementing linux command options](#Implementing-linux-command-options)
	- [cat](#cat)

## Installation

```
go get github.com/fungolang/screw
```

## Quick start

```go
package main

import (
	"fmt"
	"github.com/fungolang/screw"
)

type Hello struct {
	File string `screw:"-f; --file" usage:"file"`
}

func main() {

	h := Hello{}
	screw.SetVersion("v0.2.0")
	screw.SetAbout("This is a simple example demo")
	screw.Bind(&h)
	fmt.Printf("%#v\n", h)
}
// ./one -f test
// main.Hello{File:"test"}
// ./one --file test
// main.Hello{File:"test"}

```
## example
### base type
#### int 
```go
package main

import (
        "fmt"

        "github.com/fungolang/screw"
)

type IntDemo struct {
        Int int `screw:"short;long" usage:"int"`
}

func main() {
        id := &IntDemo{}
        screw.Bind(id)
        fmt.Printf("id = %v\n", id)
}
//  ./int -i 3
// id = &{3}
// ./int --int 3
// id = &{3}
```
#### float64
```go
package main

import (
        "fmt"

        "github.com/fungolang/screw"
)

type Float64Demo struct {
        Float64 float64 `screw:"short;long" usage:"float64"`
}

func main() {
        fd := &Float64Demo{}
        screw.Bind(fd)
        fmt.Printf("fd = %v\n", fd)
}
// ./float64 -f 3.14
// fd = &{3.14}
// ./float64 --float64 3.14
// fd = &{3.14}
```
#### duration
```go
package main

import (
        "fmt"
        "time"

        "github.com/fungolang/screw"
)

type DurationDemo struct {
        Duration time.Duration `screw:"short;long" usage:"duration"`
}

func main() {
        dd := &DurationDemo{}
        screw.Bind(dd)
        fmt.Printf("dd = %v\n", dd)
}
// ./duration -d 1h
// dd = &{1h0m0s}
// ./duration --duration 1h
// dd = &{1h0m0s}
```
#### string
```go
package main

import (
        "fmt"

        "github.com/fungolang/screw"
)

type StringDemo struct {
        String string `screw:"short;long" usage:"string"`
}

func main() {
        s := &StringDemo{}
        screw.Bind(s)
        fmt.Printf("s = %v\n", s)
}
// ./string --string hello
// s = &{hello}
// ./string -s hello
// s = &{hello}
```

## array
#### similar to curl command
```go
package main

import (
        "fmt"

        "github.com/fungolang/screw"
)

type ArrayDemo struct {
        Header []string `screw:"-H;long" usage:"header"`
}

func main() {
        h := &ArrayDemo{}
        screw.Bind(h)
        fmt.Printf("h = %v\n", h)
}
// ./array -H session:sid --header token:my
// h = &{[session:sid token:my]}
```
## similar to join command
Adding the greedy attribute supports greedy array writing. Similar to join command.
```go
package main

import (
    "fmt"

    "github.com/fungolang/screw"
)

type test struct {
    A []int `screw:"-a;greedy" usage:"test array"`
    B int   `screw:"-b" usage:"test int"`
}

func main() {
    a := &test{}
    screw.Bind(a)
    fmt.Printf("%#v\n", a)
}

/*
Run
./use_array -a 12 34 56 78 -b 100
Output
&main.test{A:[]int{12, 34, 56, 78}, B:100}
*/

```
### required flag
```go
package main

import (
	"github.com/fungolang/screw"
)

type curl struct {
	Url string `screw:"-u; --url" usage:"url" valid:"required"`
}

func main() {

	c := curl{}
	screw.Bind(&c)
}

// ./required 
// error: -u; --url must have a value!
// For more information try --help
```
#### set default value
Default values can be set using default tags, written directly to common types, and JSON for composite types
```go
package main

import (
    "fmt"
    "github.com/fungolang/screw"
)

type defaultExample struct {
    Int          int       `default:"1"`
    Float64      float64   `default:"3.64"`
    Float32      float32   `default:"3.32"`
    SliceString  []string  `default:"[\"one\", \"two\"]"`
    SliceInt     []int     `default:"[1,2,3,4,5]"`
    SliceFloat64 []float64 `default:"[1.1,2.2,3.3,4.4,5.5]"`
}

func main() {
    de := defaultExample{}
    screw.Bind(&de)
    fmt.Printf("%v\n", de) 
}
// run
//         ./use_def
// output:
//         {1 3.64 3.32 [one two] [1 2 3 4 5] [1.1 2.2 3.3 4.4 5.5]}
```
### Support environment variables
#### custom environment variable name
```go
// file name use_env.go
package main

import (
	"fmt"
	"github.com/fungolang/screw"
)

type env struct {
	OmpNumThread string `screw:"env=omp_num_thread" usage:"omp num thread"`
	Path         string `screw:"env=XPATH" usage:"xpath"`
	Max          int    `screw:"env=MAX" usage:"max thread"`
}

func main() {
	e := env{}
	screw.Bind(&e)
	fmt.Printf("%#v\n", e)
}
// run
// env XPATH=`pwd` omp_num_thread=3 MAX=4 ./use_env 
// output
// main.env{OmpNumThread:"3", Path:"/home/guo", Max:4}
```
#### Quick writing of environment variables
Using env tag generates an environment variable name based on the structure name, with the rule that the hump command name is changed to an uppercase underscore
```go
// file name use_env.go
package main

import (
	"fmt"
	"github.com/fungolang/screw"
)

type env struct {
	OmpNumThread string `screw:"env" usage:"omp num thread"`
	Xpath         string `screw:"env" usage:"xpath"`
	Max          int    `screw:"env" usage:"max thread"`
}

func main() {
	e := env{}
	screw.Bind(&e)
	fmt.Printf("%#v\n", e)
}
// run
// env XPATH=`pwd` OMP_NUM_THREAD=3 MAX=4 ./use_env 
// output
// main.env{OmpNumThread:"3", Xpath:"/home/guo", Max:4}
```
### subcommand
#### Sub command implementation method 1
```go
package main

import (
	"fmt"
	"github.com/fungolang/screw"
)

type add struct {
	All      bool     `screw:"-A; --all" usage:"add changes from all tracked and untracked files"`
	Force    bool     `screw:"-f; --force" usage:"allow adding otherwise ignored files"`
	Pathspec []string `screw:"args=pathspec"`
}

type mv struct {
	Force bool `screw:"-f; --force" usage:"allow adding otherwise ignored files"`
}

type git struct {
	Add add `screw:"subcommand=add" usage:"Add file contents to the index"`
	Mv  mv  `screw:"subcommand=mv" usage:"Move or rename a file, a directory, or a symlink"`
}

func main() {
	g := git{}
	screw.Bind(&g)
	fmt.Printf("git:%#v\n", g)
	fmt.Printf("git:set mv(%t) or set add(%t)\n", screw.IsSetSubcommand("mv"), screw.IsSetSubcommand("add"))

	switch {
	case screw.IsSetSubcommand("mv"):
		fmt.Printf("subcommand mv\n")
	case screw.IsSetSubcommand("add"):
		fmt.Printf("subcommand add\n")
	}
}

// run:
// ./git add -f

// output:
// git:main.git{Add:main.add{All:false, Force:true, Pathspec:[]string(nil)}, Mv:main.mv{Force:false}}
// git:set mv(false) or set add(true)
// subcommand add

```
#### Sub command implementation method 2
The second way to implement subcommands using screw is that the subcommand structure only implements the ```SubMain``` method, 
which the screw library will automatically call for you. It is recommended that you omit writing a bunch of 
if else judgments in main (as opposed to method 1), especially when there are a lot of subcommands.
```go
package main

import (
	"fmt"
	"github.com/fungolang/screw"
)

type add struct {
	All      bool     `screw:"-A; --all" usage:"add changes from all tracked and untracked files"`
	Force    bool     `screw:"-f; --force" usage:"allow adding otherwise ignored files"`
	Pathspec []string `screw:"args=pathspec"`
}

func (a *add) SubMain() {
	//When the add subcommand is set
	//screw automatically calls this function
}

type mv struct {
	Force bool `screw:"-f; --force" usage:"allow adding otherwise ignored files"`
}

func (m *mv) SubMain() {
	//When MV subcommand is set
	//screw automatically calls this function
}

type git struct {
	Add add `screw:"subcommand=add" usage:"Add file contents to the index"`
	Mv  mv  `screw:"subcommand=mv" usage:"Move or rename a file, a directory, or a symlink"`
}

func main() {
	g := git{}
	screw.Bind(&g)
}
```
## Get command priority
```go
package main

import (
	"fmt"
	"github.com/fungolang/screw"
)

type cat struct {
	NumberNonblank bool `screw:"-b;--number-nonblank"
                             usage:"number nonempty output lines, overrides"`

	ShowEnds bool `screw:"-E;--show-ends"
                       usage:"display $ at end of each line"`
}

func main() {

	c := cat{}
	screw.Bind(&c)

	if screw.GetIndex("number-nonblank") < screw.GetIndex("show-ends") {
		fmt.Printf("cat -b -E\n")
	} else {
		fmt.Printf("cat -E -b \n")
	}
}
// cat -be 
// output: cat -b -E
// cat -Eb
// output: cat -E -b
```


## Can only be set once
Specified options can only be set once, and error will occur if the command line option is used twice.
```go
package main

import (
    "github.com/fungolang/screw"
)

type Once struct {
    Debug bool `screw:"-d; --debug; once" usage:"debug mode"`
}

func main() {
    o := Once{}
    screw.Bind(&o)
}
/*
./once -debug -debug
error: The argument '-d' was provided more than once, but cannot be used multiple times
For more information try --help
*/
```


## quick write
Fast Writing, using fixed short, long tags to generate short, long options. It can be visually compared with [cat](#cat)  examples.
The more command-line options you have, the more time you save and efficiency you can achieve.
```go
package main

import (
    "fmt"
    "github.com/fungolang/screw"
)

type cat struct {
	NumberNonblank bool `screw:"-c;long" 
	                     usage:"number nonempty output lines, overrides"`

	ShowEnds bool `screw:"-E;long" 
	               usage:"display $ at end of each line"`

	Number bool `screw:"-n;long" 
	             usage:"number all output lines"`

	SqueezeBlank bool `screw:"-s;long" 
	                   usage:"suppress repeated empty output lines"`

	ShowTab bool `screw:"-T;long" 
	              usage:"display TAB characters as ^I"`

	ShowNonprinting bool `screw:"-v;long" 
	                      usage:"use ^ and M- notation, except for LFD and TAB" `

	Files []string `screw:"args=files"`
}

func main() {
 	c := cat{}
	err := screw.Bind(&c)

	fmt.Printf("%#v, %s\n", c, err)
}
```
## Multi structure series
Multi-structure series function. A command line view composed of multiple structures
If command-line parsing is going to take place within more than one (>=2) structure, 
you can use the structure concatenation function, which is used by the first few structures.```screw.Register()```Interface, last structure uses```screw.Bind()```function.
```go
/*
┌────────────────┐
│                │
│                │
│  ServerAddress │                        ┌─────────────────────┐
├────────────────┤                        │                     │
│                │   ──────────────────►  │                     │
│                │                        │ screw.MustRegitser()│
│     Rate       │                        │                     │
│                │                        └─────────────────────┘
└────────────────┘



┌────────────────┐
│                │
│   ThreadNum    │
│                │                        ┌─────────────────────┐
│                │                        │                     │
├────────────────┤   ──────────────────►  │                     │
│                │                        │ screw.Bind()        │
│   OpenVad      │                        │                     │
│                │                        │                     │
└────────────────┘                        └─────────────────────┘
 */

type Server struct {
	ServerAddress string `screw:"long" usage:"Server address"`
	Rate time.Duration `screw:"long" usage:"The speed at which audio is sent"`
}

type Asr struct{
	ThreadNum int `screw:"long" usage:"thread number"`
	OpenVad bool `screw:"long" usage:"open vad"`
}

 func main() {
	 asr := Asr{}
	 ser := Server{}
	 screw.MustRegister(&asr)
	 screw.Bind(&ser)
 }

 // Test the effect with the following command line parameters
 // ./example --server-address", ":8080", "--rate", "1s", "--thread-num", "20", "--open-vad"
 ```
## Support callback function parsing
* Use the notation callback=name, where name is the parsing function that needs to be called.
```go
type TestCallback struct {
	Size int `screw:"short;long;callback=ParseSize" usage:"parse size"`
	Max  int `screw:"short;long"`
}

func (t *TestCallback) ParseSize(val string) {
	//Do some parsing
	// t.Size =value after parsing
}

func main() {
 	t := TestCallback{}
	err := screw.Bind(&t)

	fmt.Printf("%#v, %s\n", t, err)
}
``` 
## Advanced features
Advanced features include some features of screw packages
### Parsing flag code to generate screw code
If your command wants to migrate to screw, but in the face of a lot of flag code, use the screw command to do everything.

#### 1.Install screw command
```bash
go get github.com/fungolang/screw/cmd/screw
```
#### 2.Resolving code containing flag packages using screw
Convert flag libraries inside main.go to screw package calls
```bash
screw -f main.go
````
```main.go```
```go
package main

import "flag"

func main() {
	s := flag.String("string", "", "string usage")
	i := flag.Int("int", "", "int usage")
	flag.Parse()
}
```

The output code is as follows
```go
package main

import (
	"github.com/fungolang/screw"
)

type flagAutoGen struct {
	Flag string `screw:"--string" usage:"string usage" `
	Flag int    `screw:"--int" usage:"int usage" `
}

func main() {
	var flagVar flagAutoGen
	screw.Bind(&flagVar)
}
```

## Implementing linux command options
### cat
```go
package main

import (
	"fmt"
	"github.com/fungolang/screw"
)

type cat struct {
	NumberNonblank bool `screw:"-c;--number-nonblank" 
	                     usage:"number nonempty output lines, overrides"`

	ShowEnds bool `screw:"-E;--show-ends" 
	               usage:"display $ at end of each line"`

	Number bool `screw:"-n;--number" 
	             usage:"number all output lines"`

	SqueezeBlank bool `screw:"-s;--squeeze-blank" 
	                   usage:"suppress repeated empty output lines"`

	ShowTab bool `screw:"-T;--show-tabs" 
	              usage:"display TAB characters as ^I"`

	ShowNonprinting bool `screw:"-v;--show-nonprinting" 
	                      usage:"use ^ and M- notation, except for LFD and TAB" `

	Files []string `screw:"args=files"`
}

func main() {

	c := cat{}
	err := screw.Bind(&c)

	fmt.Printf("%#v, %s\n", c, err)
}

/*
Usage:
    ./cat [Flags] <files> 

Flags:
    -E,--show-ends           display $ at end of each line 
    -T,--show-tabs           display TAB characters as ^I 
    -c,--number-nonblank     number nonempty output lines, overrides 
    -n,--number              number all output lines 
    -s,--squeeze-blank       suppress repeated empty output lines 
    -v,--show-nonprinting    use ^ and M- notation, except for LFD and TAB 

Args:
    <files>
*/
```
