package ports

import (
	"fmt"
	"strconv"
	"strings"
)

func Normalize(raw string) string {
	s := strings.TrimSpace(raw)
	for {
		lower := strings.ToLower(strings.TrimSpace(s))
		switch {
		case lower == "":
			return ""
		case lower == "all" || lower == "all ports":
			return ""
		case strings.HasPrefix(lower, "ports:"):
			s = strings.TrimSpace(s[len("ports:"):])
		case strings.HasPrefix(lower, "port:"):
			s = strings.TrimSpace(s[len("port:"):])
		default:
			return strings.ReplaceAll(s, " ", "")
		}
	}
}

func ParseSpecs(raw string) ([]string, error) {
	normalized := Normalize(raw)
	if normalized == "" {
		return nil, fmt.Errorf("empty port list")
	}

	parts := strings.Split(normalized, ",")
	specs := make([]string, 0, len(parts))
	for _, part := range parts {
		spec, err := parseSpec(part)
		if err != nil {
			return nil, err
		}
		if spec != "" {
			specs = append(specs, spec)
		}
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("empty port list")
	}
	return specs, nil
}

func Validate(raw string) error {
	if Normalize(raw) == "" {
		return nil
	}
	_, err := ParseSpecs(raw)
	return err
}

func parseSpec(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}

	if strings.Contains(s, "-") || strings.Contains(s, ":") {
		sep := "-"
		if strings.Contains(s, ":") {
			sep = ":"
		}
		pieces := strings.Split(s, sep)
		if len(pieces) != 2 {
			return "", fmt.Errorf("invalid port range %q", raw)
		}
		start, err := parsePort(pieces[0])
		if err != nil {
			return "", err
		}
		end, err := parsePort(pieces[1])
		if err != nil {
			return "", err
		}
		if start > end {
			return "", fmt.Errorf("invalid port range %q: start is greater than end", raw)
		}
		return fmt.Sprintf("%d:%d", start, end), nil
	}

	port, err := parsePort(s)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(port), nil
}

func parsePort(raw string) (int, error) {
	s := strings.TrimSpace(raw)
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q", raw)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port %d out of range", port)
	}
	return port, nil
}
