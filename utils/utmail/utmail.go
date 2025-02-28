package utmail

import (
	"regexp"
	"strings"
)

// ValidEmail is a function that validates if the input string is a valid email address.
func ValidEmail(input string) bool {
	pattern := `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	re := regexp.MustCompile(pattern)
	return re.MatchString(input)
}

// NormalizeEmail normalizes an email address
func NormalizeEmail(input string) string {
	rulePlusOnly := regexp.MustCompile(`\+.*$`)
	rulePlusAndDot := regexp.MustCompile(`\.|\+.*$`)

	normalizableProviders := map[string]struct {
		rule    *regexp.Regexp
		aliasOf string
	}{
		//"gmail.com":      {rule: rulePlusAndDot}, TODO: Current pass for email .
		"googlemail.com": {rule: rulePlusAndDot, aliasOf: "gmail.com"},
		"hotmail.com":    {rule: rulePlusOnly},
		"live.com":       {rule: rulePlusAndDot},
		"outlook.com":    {rule: rulePlusOnly},
	}

	email := strings.ToLower(input)
	emailParts := strings.SplitN(email, "@", 2)

	if len(emailParts) != 2 {
		return input
	}

	username, domain := emailParts[0], emailParts[1]

	if provider, ok := normalizableProviders[domain]; ok {
		username = provider.rule.ReplaceAllString(username, "")
		if provider.aliasOf != "" {
			domain = provider.aliasOf
		}
	}

	return username + "@" + domain
}
