package main

import (
	"fmt"
	"hash/fnv"
	"os"
	"regexp"
	"strings"
	"time"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/ysmood/kit"
)

type options struct {
	patterns    *[]string
	dir         *string
	cmd         []string
	prefix      *string
	clearScreen *bool
	noInitRun   *bool
	raw         *bool
	poll        *time.Duration
	debounce    *time.Duration
}

func main() {
	optsList := []*options{}
	for _, args := range split(argsFromConfigFile(os.Args[1:]), "---") {
		optsList = append(optsList, genOptions(args))
	}

	fns := []func(){}
	for _, opts := range optsList {
		fns = append(fns, func(opts *options) func() {
			return func() {
				guard :=
					kit.Guard(opts.cmd...).
						Patterns(filterEmpty(*opts.patterns)...).
						Debounce(opts.debounce).
						Interval(opts.poll).
						ExecCtx(
							kit.Exec().
								Dir(*opts.dir).
								Raw().
								Prefix(genPrefix(*opts.prefix, opts.cmd)),
						)

				if *opts.clearScreen {
					guard.ClearScreen()
				}

				if *opts.noInitRun {
					guard.NoInitRun()
				}

				guard.MustDo()
			}
		}(opts))
	}
	kit.All(fns...)()
}

func genOptions(args []string) *options {
	opts := &options{}

	app := kingpin.New(
		"guard",
		`run and guard a command, kill and rerun it when watched files are modified

		Examples:

		 # follow the "--" is the command and its arguments you want to execute
		 # kill and restart the web server when a file changes
		 guard -- node server.js

		 # use ! prefix to ignore pattern, the below means watch all files but not those in tmp dir
		 guard -w '**' -w '!tmp/**' -- echo changed

		 # the special !g pattern will read the gitignore files and ignore patterns in them
		 # the below is the default patterns guard will use
		 guard -w '**' -w '!g' -- echo changed

		 # support go template
		 guard -- echo {{op}} {{path}} {{file}}

		 # watch and sync current dir to another machine
		 guard -n -- rsync {{path}} root@host:/home/me/app/{{path}}
		 guard -n -- docker cp {{path}} my-container:/app/{{path}}

		 # the patterns must be quoted
		 guard -w '*.go' -w 'lib/**/*.go' -- go run main.go

		 # the output will be prefix with red 'my-app | '
		 guard -p 'my-app | @red' -- python test.py

		 # use "---" as separator to guard multiple commands
		 guard -w 'a/*' -- ls a --- -w 'b/*' -- ls b
		`,
	)
	opts.patterns = app.Flag("watch", "the pattern to watch, can set multiple patterns").Short('w').Strings()
	opts.dir = app.Flag("dir", "base dir path").Short('d').String()
	opts.prefix = app.Flag("prefix", "prefix for command output").Short('p').Default("auto").String()
	opts.clearScreen = app.Flag("clear-screen", "clear screen before each run").Short('c').Bool()
	opts.noInitRun = app.Flag("no-init-run", "don't execute the cmd on startup").Short('n').Bool()
	opts.poll = app.Flag("poll", "poll interval").Default("300ms").Duration()
	opts.debounce = app.Flag("debounce", "suppress the frequency of the event").Default("300ms").Duration()
	opts.raw = app.Flag("raw", "when you need to interact with the subprocess").Bool()

	app.Version(kit.Version)

	args, cmdArgs := parseArgs(args)

	_, err := app.Parse(args)

	if err != nil {
		fmt.Println("for help run: guard --help")
		panic(err)
	}

	if cmdArgs == nil {
		panic("empty command")
	}

	opts.cmd = cmdArgs

	return opts
}

func filterEmpty(list []string) []string {
	newList := []string{}
	for _, el := range list {
		if el != "" {
			newList = append(newList, el)
		}
	}
	return newList
}

func parseArgs(args []string) (appArgs []string, cmdArgs []string) {
	i := indexOf(args, "--")

	if i == -1 {
		return args, nil
	}

	return args[:i], args[i+1:]
}

func indexOf(list []string, str string) int {
	for i, elem := range list {
		if elem == str {
			return i
		}
	}

	return -1
}

func argsFromConfigFile(args []string) []string {
	for _, elem := range args {
		if len(elem) > 1 && elem[0] == '@' {
			f, err := kit.ReadFile(elem[1:])
			if err != nil {
				return args
			}
			return regexp.MustCompile(`[\n\r]+`).Split(string(f), -1)
		}
	}
	return args
}

func genPrefix(prefix string, args []string) string {
	if prefix == "auto" {
		h := fnv.New32a()
		kit.E(h.Write([]byte(strings.Join(args, ""))))

		return kit.C(fmt.Sprint(args[0], " | "), fmt.Sprint(h.Sum32()%256))
	}

	return prefix
}

func split(args []string, sep string) [][]string {
	list := [][]string{}

	tmp := []string{}
	for _, arg := range args {
		if arg == "---" {
			list = append(list, tmp)
			tmp = []string{}
			continue
		}
		tmp = append(tmp, arg)
	}

	return append(list, tmp)
}
