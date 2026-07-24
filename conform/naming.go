package conform

import (
	"regexp"
	"strings"
	"unicode"
)

var namingCamelPattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)

// namingCamelOk reports whether a name is camelCase — the only casing allowed
// to cross the wire toward the frontend (BFD-11).
func namingCamelOk(name string) bool {
	return namingCamelPattern.MatchString(name)
}

// namingPluralIrregular maps irregular plurals to the regular form BFD-13
// demands. You are speaking to computers, not writing prose.
var namingPluralIrregular = map[string]string{
	"people":     "persons",
	"children":   "childs",
	"men":        "mans",
	"women":      "womans",
	"feet":       "foots",
	"teeth":      "tooths",
	"geese":      "gooses",
	"mice":       "mouses",
	"oxen":       "oxes",
	"cacti":      "cactuses",
	"fungi":      "funguses",
	"nuclei":     "nucleuses",
	"syllabi":    "syllabuses",
	"alumni":     "alumnuses",
	"criteria":   "criterions",
	"phenomena":  "phenomenons",
	"indices":    "indexes",
	"matrices":   "matrixes",
	"vertices":   "vertexes",
	"appendices": "appendixes",
	"leaves":     "leafs",
	"wolves":     "wolfs",
	"knives":     "knifes",
	"wives":      "wifes",
	"lives":      "lifes",
	"halves":     "halfs",
	"shelves":    "shelfs",
}

type namingPluralViolation struct {
	Word    string
	Regular string
}

// namingPluralFind reports the first irregular plural word inside an
// identifier, path segment, or schema name.
func namingPluralFind(identifier string) (namingPluralViolation, bool) {
	for _, word := range namingWords(identifier) {
		if regular, found := namingPluralIrregular[word]; found {
			return namingPluralViolation{Word: word, Regular: regular}, true
		}
	}
	return namingPluralViolation{}, false
}

// namingWords splits an identifier into lowercase words, honoring camelCase,
// snake_case, and kebab-case boundaries alike.
func namingWords(identifier string) []string {
	words := []string{}
	current := strings.Builder{}
	flush := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	for _, r := range identifier {
		switch {
		case unicode.IsUpper(r):
			flush()
			current.WriteRune(unicode.ToLower(r))
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			current.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return words
}
