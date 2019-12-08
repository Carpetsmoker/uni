// Command uni prints Unicode information about characters.
package main // import "arp242.net/uni"

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"arp242.net/uni/unidata"
)

var (
	errFlag      = errors.New("")
	errNoMatches = errors.New("no matches")
)

var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
	exit             = os.Exit
)

const usagetext = `Usage: %s [-hrq] [identify | search | print | emoji]

Flags:
    -h      Show this help.
    -q      Quiet output; don't print header, "no matches", etc.
    -r      "Raw" unprocessed output; default is to display graphical variants
            for control characters and display ◌ (U+25CC) before combining
            characters. Note control characters may mangle the output.

Commands:
    identify [string string ...]
        Idenfity all the characters in the given strings.

    search [word word ...]
        Search description for any of the words.

    print [ident ident ...]
        Print characters by codepoint, category, or block, or special name:

            Codepoint    U+2042, U+2042..U+2050
            Category     OtherPunctuation, Po
            Block        GeneralPunctuation
            all          Everything

        Names are matched case insensitive. Spaces and commas are optional and
        can be replaced with an underscore. "Po", "po", "punction, OTHER",
        "Punctuation_other", and PunctuationOther are all identical.

    emoji [-tone tone] [word word ...]
        Print emojis by group name:

             all              Everything.
             groups           All group and subgroup names.
             <anything else>  Emojis matching the group or subgroup.

        The skin tone modifier is applied on supported emojies if -tone is
        given. Supported tones: light, mediumlight, medium, mediumdark, dark.
        Note: emojis may consist of multiple codepoints!
`

func usage(err error) {
	out := stdout
	e := 0
	if err != nil {
		out = stderr
		e = 1
		if err != errFlag {
			_, _ = fmt.Fprintf(out, "%s: error: %v\n", os.Args[0], err)
		}
	}

	_, _ = fmt.Fprintf(out, usagetext, os.Args[0])
	exit(e)
}

func main() {
	var (
		//output string
		quiet bool
		help  bool
		raw   bool
	)
	// TODO: Output format; valid values are human (default), csv, tsv, json.
	// TODO: Add option to configure columns.
	//flag.StringVar(&output, "o", "human", "")
	flag.BoolVar(&quiet, "q", false, "")
	flag.BoolVar(&help, "h", false, "")
	flag.BoolVar(&raw, "r", false, "")
	flag.Usage = func() { usage(errFlag) }
	flag.Parse()

	if help {
		usage(nil)
	}

	args := flag.Args()
	if len(args) == 0 {
		usage(errors.New("no command given"))
	}

	var err error
	switch strings.ToLower(args[0]) {
	default:
		usage(fmt.Errorf("unknown command: %q", args[0]))
	case "help", "h":
		usage(nil)
	case "identify", "i":
		err = identify(getargs(args[1:], quiet), quiet, raw)
	case "search", "s":
		err = search(getargs(args[1:], quiet), quiet, raw)
	case "print", "p":
		err = print(getargs(args[1:], quiet), quiet, raw)
	case "emoji", "e":
		err = emoji(getargs(args[1:], quiet), quiet, raw)
	}
	if err == errNoMatches && quiet {
		err = nil
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		exit(1)
	}
}

// Use commandline args or stdin.
func getargs(args []string, quiet bool) []string {
	if len(args) > 0 {
		return args
	}

	if !quiet {
		// TODO: clear with \r? Hmm..
		fmt.Fprintf(stderr, "uni: reading from stdin...\n")
	}
	stdin, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(fmt.Errorf("read stdin: %s", err))
	}
	return strings.Split(strings.TrimRight(string(stdin), "\n"), "\n")
}

func search(args []string, quiet, raw bool) error {
	var na []string
	for _, a := range args {
		if a != "" {
			na = append(na, a)
		}
	}
	args = na
	if len(args) == 0 {
		return errors.New("search: need search term")
	}

	var out printer
	words := make([]string, len(args))
	for i := range args {
		words[i] = strings.ToUpper(args[i])
	}
	for _, info := range unidata.Codepoints {
		m := 0
		for _, w := range words {
			if strings.Contains(info.Name, w) {
				m++
			}
		}
		if m == len(words) {
			out = append(out, info)
		}
	}

	if len(out) == 0 {
		return errNoMatches
	}

	out.PrintSorted(stdout, quiet, raw)
	return nil
}

// TODO: treat man/women thing as modifier too; I don't really care much about
// having "person shrugging", "man shrugging", and "women shrugging" all turn up
// in the results for shrugging.
//
//   $ uni e                             # Default: show only "person" w/o skin modifiers.
//   $ uni e -tone dark                  # Apply skin modifer.
//
//   $ uni e -gender man                 # Show only "man" variants
//   $ uni e -gender man,women           # Show both man and women, but not "person"
//   $ uni e -gender man,women,person    # Show all.
//
//   $ uni e -tone dark -gender women    # Show women and apply dark skin modifier.
func emoji(args []string, quiet, raw bool) error {
	subflag := flag.NewFlagSet("emoji", flag.ExitOnError)
	tone := subflag.String("tone", "", "Skin tone; light, mediumlight, medium, mediumdark, or dark")
	subflag.Parse(args)

	switch *tone {
	case "":
	case "light":
		*tone = "\U0001f3fb"
	case "mediumlight":
		*tone = "\U0001f3fc"
	case "medium":
		*tone = "\U0001f3fd"
	case "mediumdark":
		*tone = "\U0001f3fe"
	case "dark":
		*tone = "\U0001f3ff"
	default:
		fmt.Fprintf(stderr, "uni: invalid skin tone: %q\n", *tone)
		flag.Usage()
		exit(55)
	}

	out := [][]string{}
	cols := []int{4, 0, 0, 0}
	for _, a := range subflag.Args() {
		a = strings.ToLower(a)
		switch a {
		case "all":
			a = ""
		case "groups":
			for _, g := range unidata.EmojiGroups {
				fmt.Fprintln(stdout, g)
				for _, sg := range unidata.EmojiSubgroups[g] {
					fmt.Fprintln(stdout, "   ", sg)
				}
			}
			return nil
		}

		found := false
		for _, e := range unidata.Emojis {
			if !strings.Contains(strings.ToLower(e.Group), a) &&
				!strings.Contains(strings.ToLower(e.Subgroup), a) {
				continue
			}

			found = true

			c := e.String()
			if *tone != "" && e.SkinTones {
				c += "\u200d" + *tone
			}

			out = append(out, []string{c, e.Name, e.Group, e.Subgroup})
			if l := utf8.RuneCountInString(e.Name); l > cols[1] {
				cols[1] = l
			}
			if l := utf8.RuneCountInString(e.Group); l > cols[2] {
				cols[2] = l
			}
			if l := utf8.RuneCountInString(e.Subgroup); l > cols[3] {
				cols[3] = l
			}
		}

		if !found {
			return fmt.Errorf("no such emoji group or subgroup: %q", a)
		}
	}

	// TODO: not always correctly aligned as some emojis are double-width and
	// some are not. As far as I can tell, there is no good way to predict this
	// as it will depend on the font. Unicode recommends "emoji presentation
	// sequences behave as though they were East Asian Wide", but that's too
	// simplistic too.
	for _, o := range out {
		for i, c := range o {
			if i == 0 {
				fmt.Fprintf(stdout, c+" ")
			} else {
				fmt.Fprint(stdout, fill(c, cols[i]+2))
			}
		}
		fmt.Fprintln(stdout, "")
	}
	return nil
}

func print(args []string, quiet, raw bool) error {
	var out printer

	for _, a := range args {
		canon := unidata.CanonicalCategory(a)

		// Print everything.
		if canon == "all" {
			for _, info := range unidata.Codepoints {
				out = append(out, info)
			}
			continue
		}

		// Category name.
		if cat, ok := unidata.Catmap[canon]; ok {
			for _, info := range unidata.Codepoints {
				if info.Cat == cat {
					out = append(out, info)
				}
			}
			continue
		}

		// Block.
		if bl, ok := unidata.Blockmap[canon]; ok {
			for cp := unidata.Blocks[bl][0]; cp <= unidata.Blocks[bl][1]; cp++ {
				s, ok := unidata.Codepoints[fmt.Sprintf("%04X", cp)]
				if ok {
					out = append(out, s)
				}
			}
			continue
		}

		// U2042, U+2042, U+2042..U+2050, 2042..2050
		if strings.HasPrefix(canon, "u") || strings.Contains(canon, "..") {
			canon = strings.ToUpper(canon)

			s := strings.Split(canon, "..")
			switch len(s) {
			case 1:
				s = append(s, s[0])
			case 2:
				// Do nothing
			default:
				return fmt.Errorf("unknown ident: %q", a)
			}

			start, err := strconv.ParseInt(strings.TrimLeft(strings.TrimLeft(s[0], "U"), "+"), 16, 64)
			if err != nil {
				return err
			}
			end, err := strconv.ParseInt(strings.TrimLeft(strings.TrimLeft(s[1], "U"), "+"), 16, 64)
			if err != nil {
				return err
			}

			for i := start; i <= end; i++ {
				info, ok := unidata.FindCodepoint(rune(i))
				if !ok {
					return fmt.Errorf("unknown codepoint: U+%.4X", i)
				}
				out = append(out, info)
			}

			continue
		}

		return fmt.Errorf("unknown identifier: %q", a)
	}

	out.PrintSorted(stdout, quiet, raw)
	return nil
}

func identify(ins []string, quiet, raw bool) error {
	in := strings.Join(ins, "")
	if !utf8.ValidString(in) {
		_, _ = fmt.Fprintf(stderr, "uni: WARNING: input string is not valid UTF-8\n")
	}

	var out printer
	for _, c := range in {
		info, ok := unidata.FindCodepoint(c)
		if !ok {
			return fmt.Errorf("unknown codepoint: %.4X", c)
		}

		out = append(out, info)
	}

	out.Print(stdout, quiet, raw)
	return nil
}

func fmtChar(c rune, raw bool) string {
	if raw {
		return string(c)
	}

	// Display combining characters with ◌.
	if unicode.In(c, unicode.Mn, unicode.Mc, unicode.Me) {
		return "\u25cc" + string(c)
	}

	switch {
	case unicode.IsControl(c):
		switch {
		case c < 0x20: // C0; use "Control Pictures" block
			c += 0x2400
		case c == 0x7f: // DEL
			c = 0x2421
		// No control pictures for C1 or anything else, use "open box".
		default:
			c = 0x2423
		}
	// "Other, Format" category except the soft hyphen and spaces.
	case !unicode.IsPrint(c) && c != 0x00ad && !unicode.In(c, unicode.Zs):
		c = 0xfffd
	}

	return string(c)
}
