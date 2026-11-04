package auth

import "strings"

func NormalizePhone(value string) (string, error) {
	if len(value) > 96 {
		return "", ErrInvalid
	}
	var phone strings.Builder
	for _, r := range strings.TrimSpace(value) {
		switch {
		case r >= '۰' && r <= '۹':
			r = '0' + r - '۰'
		case r >= '٠' && r <= '٩':
			r = '0' + r - '٠'
		}
		if r == ' ' || r == '-' || r == '(' || r == ')' {
			continue
		}
		if r != '+' && (r < '0' || r > '9') {
			return "", ErrInvalid
		}
		phone.WriteRune(r)
	}
	normalized := phone.String()
	switch {
	case strings.HasPrefix(normalized, "0098"):
		normalized = "+98" + normalized[4:]
	case strings.HasPrefix(normalized, "0"):
		normalized = "+98" + normalized[1:]
	case strings.HasPrefix(normalized, "9"):
		normalized = "+98" + normalized
	}
	if len(normalized) != 13 || !strings.HasPrefix(normalized, "+989") {
		return "", ErrInvalid
	}
	for _, r := range normalized[4:] {
		if r < '0' || r > '9' {
			return "", ErrInvalid
		}
	}
	return normalized, nil
}

func normalizeCode(value string) (string, error) {
	var code strings.Builder
	for _, r := range value {
		switch {
		case r >= '۰' && r <= '۹':
			r = '0' + r - '۰'
		case r >= '٠' && r <= '٩':
			r = '0' + r - '٠'
		}
		if r < '0' || r > '9' {
			return "", ErrInvalid
		}
		code.WriteRune(r)
	}
	if code.Len() != 6 {
		return "", ErrInvalid
	}
	return code.String(), nil
}
