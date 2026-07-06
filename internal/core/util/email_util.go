package util

import (
	"net/mail"
	"regexp"
	"strings"

	"github.com/itsLeonB/ezutil/v2"
)

func IsValidEmail(e string) bool {
	_, err := mail.ParseAddress(e)
	return err == nil
}

var re = regexp.MustCompile(`[a-zA-Z]+`)

func GetNameFromEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) < 2 || parts[0] == "" {
		return ""
	}
	localPart := parts[0]

	matches := re.FindAllString(localPart, -1)
	if len(matches) > 0 {
		name := matches[0]
		return ezutil.Capitalize(name)
	}

	return ""
}
