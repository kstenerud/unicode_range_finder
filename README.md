Unicode Range Finder
====================

A quick utility to find and print ranges of unicode codepoints. I wrote this to help write Unicode ranges for BNF since they can get pretty hairy depending on what you want to match.

For example, to get the range for printable, non-control characters:

    ./unicode_range_finder -leadup="    char ::= " cat=Cf cat=L cat=M cat=N cat=P cat=S cat=Z cp=09 cp=0a cp=0d

... which produces 699 range groups. Care to generate those by hand? ;-)


Usage
-----

```
Usage: unicode_range_finder [options] <search params>

Options:
  -highcol int
      Highest column to print at (columns start at 1) (default 80)
  -leadup string
      Leadup text to print and align to
  -range string
      Range of codepoints to search, or range to build if -unicode specified (e.g. 50-0x7f)
  -unicode string
      Regenerate generated.go from /path/to/ucd.all.flat.xml. Get it from https://www.unicode.org/Public/UCD/latest/ucdxml/ucd.all.flat.zip

Where search params is a space separated set of:
 * A category (e.g. cat=N, cat=Cc etc)
 * A specific or range of characters (e.g. ch=a-z, ch=# etc)
 * A specific or range of codepoints (e.g. cp=1a-af, cp=feff etc)

Categories:
    Cc: Control
    Cf: Format
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
```


Warning about editing the source
--------------------------------

The file `generated.go` is quite big (85mb), which will cause electron based apps like VS Code to peg ALL of your CPUs and chew through ALL of your memory (verified on a 32GB system) whenever you load, save, or modify a go source file (whether you open `generated.go` or not). Your system will slow to a crawl.

As a workaround, you can temporarily generate a smaller `generated.go` with something like `./unicode_range_finder -range=0-1000 -unicode /path/to/ucd.all.flat.xml` to make VS Code behave better while you're editing and testing the source.
