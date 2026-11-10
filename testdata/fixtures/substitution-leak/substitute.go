package substitutionleak

import "strings"

var replacement = "and"

func normalize(input string) string { return strings.ReplaceAll(input, "&", replacement) }
