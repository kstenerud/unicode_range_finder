// Copyright 2022 Karl Stenerud
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
// IN THE SOFTWARE.

// Package build generates code for other parts of the library. The lack of
// generics and inheritance makes a number of things tedious and error prone,
// which these generators attempt to deal with.
package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func main() {
	flag.Usage = printUsage
	unicodePath := flag.String("unicode", "", "Regenerate generated.go from /path/to/ucd.all.flat.xml. Get it from https://www.unicode.org/Public/UCD/latest/ucdxml/ucd.all.flat.zip")
	leadup := flag.String("leadup", "", "Leadup text to print and align to")
	highCol := flag.Int("highcol", 80, "Highest column to print at (columns start at 1)")
	rangeStr := flag.String("range", "", "Range of codepoints to search, or range to build if -unicode specified (e.g. 50-0x7f)")
	flag.Parse()

	lowCP := uint64(0)
	highCP := uint64(0x10ffff)
	if len(*rangeStr) > 0 {
		r := strings.Split(*rangeStr, "-")
		if len(r) != 2 {
			panic(fmt.Errorf("malformed range [%v]", *rangeStr))
		}
		var err error
		lowCP, err = strconv.ParseUint(r[0], 0, 32)
		if err != nil {
			panic(err)
		}
		highCP, err = strconv.ParseUint(r[1], 0, 32)
		if err != nil {
			panic(err)
		}
	}

	if *unicodePath != "" {
		generateCode(lowCP, highCP, *unicodePath)
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Printf("Must provide at least one query param")
		printUsage()
		return
	}
	sb := strings.Builder{}
	for i, arg := range args {
		if i > 0 {
			sb.WriteRune(' ')
		}
		sb.WriteString(arg)
	}
	q := sb.String()

	inclusiveMatchers := parseMatchers(q)
	var exclusiveMatchers []CodepointMatcher
	if lowCP > 0 || highCP < 0x10ffff {
		exclusiveMatchers = append(exclusiveMatchers, func(cp Codepoint) bool {
			return cp.Codepoint < rune(lowCP) || cp.Codepoint > rune(highCP)
		})
	}

	ranges := query(func(cp Codepoint) bool {
		for _, matcher := range inclusiveMatchers {
			if matcher(cp) {
				return true
			}
		}
		return false
	}, func(cp Codepoint) bool {
		for _, matcher := range exclusiveMatchers {
			if matcher(cp) {
				return true
			}
		}
		return false
	})

	printRanges(ranges, *leadup, *highCol)
}

func printRanges(ranges []Range, leadup string, highCol int) {
	if highCol <= 0 {
		fmt.Printf("%v%v\n", leadup, Ranges(ranges))
		return
	}

	for len(ranges) > 0 {
		fmt.Print(leadup)
		ranges = ranges[printLine(ranges, len(leadup), highCol):]
		leadup = strings.Repeat(" ", len(leadup))
	}
}

func printLine(ranges []Range, lowCol int, highCol int) (entriesUsed int) {
	sb := strings.Builder{}
	col := lowCol
	for i, r := range ranges {
		sb.Reset()
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(r.String())
		if i < len(ranges)-1 {
			sb.WriteString(" |")
		}
		str := sb.String()
		col += len(str)
		if col > highCol {
			break
		}
		fmt.Print(str)
		entriesUsed++
	}
	fmt.Println("")
	return
}

func printUsage() {
	fmt.Println("Usage: unicode_range_finder [options] <search params>")

	fmt.Println("\nOptions:")
	flag.PrintDefaults()

	fmt.Printf(`
Where search params is a space separated set of:
 * A category (e.g. cat=N, cat=Cc etc)
 * A specific or range of characters (e.g. ch=a-z, ch=# etc)
 * A specific or range of codepoints (e.g. cp=1a-af, cp=feff etc)

Categories:
    Cc: Control
    Cf: Format
    Cn: Not Assigned
    Co: Private Use
    Cs: Surrrogate
    Ll: Lowercase Letter
    Lm: Modifier Letter
    Lo: Other Letter
    Lt: Titlecase Letter
    Lu: Uppercase Letter
    Mc: Spacing Mark
    Me: Enclosing Mark
    Mn: Nonspacing Mark
    Nd: Decimal Number
    Nl: Letter Number
    No: Other Number
    Pc: Connector Punctuation
    Pd: Dash Punctuation
    Pe: Close Punctuation
    Pf: Final Punctuation
    Pi: Initial Punctuation
    Po: Other Punctuation
    Ps: Open Punctuation
    Sc: Currency Symbol
    Sk: Modifier Symbol
    Sm: Math Symbol
    So: Other Symbol
    Zl: Line Separator
    Zp: Paragraph Separator
    Zs: Space Separator
`)
}

func parseMatchers(matchers string) (cpMatchers []CodepointMatcher) {
	for _, matcher := range strings.Split(matchers, " ") {
		if len(matcher) == 0 {
			continue
		}

		args := strings.Split(matcher, "=")
		if len(args) != 2 {
			fmt.Printf("%v: Unknown query param", matcher)
			printUsage()
			return
		}
		switch args[0] {
		case "cat":
			cat := args[1]
			if len(cat) == 1 {
				majorCategory := cat[0]
				cpMatchers = append(cpMatchers, func(cp Codepoint) bool {
					return cp.MajorCategory == majorCategory
				})
				continue
			}

			if len(cat) == 2 {
				majorCategory := cat[0]
				minorCategory := cat[1]
				cpMatchers = append(cpMatchers, func(cp Codepoint) bool {
					return cp.MajorCategory == majorCategory && cp.MinorCategory == minorCategory
				})
				continue
			}
		case "ch":
			ch := args[1]
			lowHi := strings.Split(ch, "-")
			if len(lowHi) == 1 {
				code, _ := utf8.DecodeRuneInString(ch)
				cpMatchers = append(cpMatchers, func(cp Codepoint) bool {
					return cp.Codepoint == code
				})
			} else {
				low, _ := utf8.DecodeRuneInString(lowHi[0])
				hi, _ := utf8.DecodeRuneInString(lowHi[1])
				cpMatchers = append(cpMatchers, func(cp Codepoint) bool {
					return cp.Codepoint >= low && cp.Codepoint <= hi
				})
			}
			continue
		case "cp":
			cp := args[1]
			lowHi := strings.Split(cp, "-")
			if len(lowHi) == 1 {
				code, err := strconv.ParseUint(args[1], 16, 32)
				if err != nil {
					panic(err)
				}
				cpMatchers = append(cpMatchers, func(cp Codepoint) bool {
					return cp.Codepoint == rune(code)
				})
			} else {
				lowCode, err := strconv.ParseUint(lowHi[0], 16, 32)
				if err != nil {
					panic(err)
				}
				hiCode, err := strconv.ParseUint(lowHi[1], 16, 32)
				if err != nil {
					panic(err)
				}
				cpMatchers = append(cpMatchers, func(cp Codepoint) bool {
					return cp.Codepoint >= rune(lowCode) && cp.Codepoint <= rune(hiCode)
				})
			}
			continue
		}
		fmt.Printf("%v: Unknown query param", matcher)
		printUsage()
		return
	}
	return
}

type CodepointMatcher func(Codepoint) bool

func query(inclusiveMatcher CodepointMatcher, exclusiveMatcher CodepointMatcher) (ranges Ranges) {
	inRange := false
	var startRange rune
	for i, cp := range allCodepoints {
		if inclusiveMatcher(cp) && !exclusiveMatcher(cp) {
			if !inRange {
				startRange = rune(i)
			}
			inRange = true
		} else {
			if inRange {
				ranges = append(ranges, Range{
					Begin: startRange,
					End:   rune(i - 1),
				})
			}
			inRange = false
		}
	}
	if inRange {
		ranges = append(ranges, Range{
			Begin: startRange,
			End:   allCodepoints[len(allCodepoints)-1].Codepoint,
		})
	}
	return
}

type Range struct {
	Begin rune
	End   rune
}

func (_this *Range) String() string {
	if _this.Begin == _this.End {
		return fmt.Sprintf("#x%X", _this.Begin)
	}
	return fmt.Sprintf("[#x%X-#x%X]", _this.Begin, _this.End)
}

type Ranges []Range

func (_this Ranges) String() string {
	sb := strings.Builder{}
	for i, r := range _this {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(r.String())
	}
	return sb.String()
}

// Uncomment when deleting generated.go
// var allCodepoints []Codepoint

func generateCode(lowCP uint64, highCP uint64, unicodePath string) {
	generatedFilename := "generated.go"

	codepoints := make([]Codepoint, 0, 0x110000)
	for i := 0; i < 0x110000; i++ {
		codepoints = append(codepoints, Codepoint{
			MajorCategory: ' ',
			MinorCategory: ' ',
			Codepoint:     rune(i),
		})
	}

	loadedCodepoints, err := loadUnicodeDB(unicodePath)
	if err != nil {
		panic(err)
	}
	for _, cp := range loadedCodepoints {
		cp.fixup()
		codepoints[cp.Codepoint] = Codepoint{
			MajorCategory: cp.MajorCategory,
			MinorCategory: cp.MinorCategory,
			Codepoint:     cp.Codepoint,
		}
	}

	if highCP < uint64(len(codepoints)) {
		codepoints = codepoints[:highCP]
	}
	if lowCP > 0 {
		codepoints = codepoints[lowCP:]
	}

	sb := strings.Builder{}
	sb.WriteString(`package main

var allCodepoints = []Codepoint{
`)

	for _, codepoint := range codepoints {
		sb.WriteString(fmt.Sprintf(`	{
		MajorCategory: '%c',
		MinorCategory: '%c',
		Codepoint:     0x%x,
	},
`, codepoint.MajorCategory, codepoint.MinorCategory, codepoint.Codepoint))
	}

	sb.WriteString(`
}
`)

	os.Remove(generatedFilename)
	if err := os.WriteFile(generatedFilename, []byte(sb.String()), 0644); err != nil {
		panic(err)
	}
}

type Codepoint struct {
	Codepoint     rune
	MajorCategory byte
	MinorCategory byte
}

func loadUnicodeDB(path string) (codepoints LoadedCodepointSet, err error) {
	document, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var dbWrapper DBWrapper
	if err = xml.Unmarshal(document, &dbWrapper); err != nil {
		return
	}

	codepoints = make(LoadedCodepointSet, 0, len(dbWrapper.DB.Chars))
	for _, codepoint := range dbWrapper.DB.Chars {
		codepoints = append(codepoints, codepoint.All()...)
	}
	for _, char := range dbWrapper.DB.Reserveds {
		codepoints = append(codepoints, char.All()...)
	}

	sort.Slice(codepoints, func(i, j int) bool {
		return codepoints[i].Codepoint < codepoints[j].Codepoint
	})

	return
}

type LoadedCodepointSet []*LoadedCodepoint

func (_this LoadedCodepointSet) RunesWithCriteria(criteria func(*LoadedCodepoint) bool) (runes []rune) {
	for _, char := range _this {
		if criteria(char) {
			runes = append(runes, rune(char.Codepoint))
		}
	}
	return
}

type DBWrapper struct {
	XMLName xml.Name   `xml:"ucd"`
	DB      *UnicodeDB `xml:"repertoire"`
}

type UnicodeDB struct {
	XMLName   xml.Name           `xml:"repertoire"`
	Chars     []*LoadedCodepoint `xml:"char"`
	Reserveds []*LoadedCodepoint `xml:"reserved"`
}

func (_this *UnicodeDB) PerformAction(criteria func(*LoadedCodepoint) bool, action func(*LoadedCodepoint)) {
	for _, char := range _this.Chars {
		if criteria(char) {
			action(char)
		}
	}
}

type LoadedCodepoint struct {
	CodepointStr  string `xml:"cp,attr"`
	FirstCPStr    string `xml:"first-cp,attr"`
	LastCPStr     string `xml:"last-cp,attr"`
	Category      string `xml:"gc,attr"`
	MajorCategory byte
	MinorCategory byte
	Codepoint     rune
}

func (_this *LoadedCodepoint) fixup() {
	if _this.MinorCategory < 'a' || _this.MinorCategory > 'z' {
		_this.MinorCategory = ' '
	}
	if _this.MajorCategory < 'A' || _this.MajorCategory > 'Z' {
		_this.MajorCategory = ' '
	}

}

func (_this *LoadedCodepoint) All() (result []*LoadedCodepoint) {
	_this.MajorCategory = _this.Category[0]

	if len(_this.Category) >= 2 {
		_this.MinorCategory = byte(_this.Category[1])
	}

	if _this.CodepointStr != "" {
		codepoint, err := strconv.ParseInt(_this.CodepointStr, 16, 32)
		if err != nil {
			return
		}
		_this.Codepoint = rune(codepoint)
		return []*LoadedCodepoint{_this}
	}

	firstCP, err := strconv.ParseInt(_this.FirstCPStr, 16, 32)
	if err != nil {
		return
	}
	lastCP, err := strconv.ParseInt(_this.LastCPStr, 16, 32)
	if err != nil {
		return
	}

	for i := rune(firstCP); i <= rune(lastCP); i++ {
		result = append(result, &LoadedCodepoint{
			Category:      _this.Category,
			MajorCategory: _this.MajorCategory,
			MinorCategory: _this.MinorCategory,
			Codepoint:     i,
		})
	}

	return
}
