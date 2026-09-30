// Package validate checks gRPC request fields at the service boundary, returning
// InvalidArgument so the gateway answers 400.
package validate

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Required fails on the first blank value; fields are name, value pairs.
func Required(fields ...string) error {
	for i := 0; i+1 < len(fields); i += 2 {
		if strings.TrimSpace(fields[i+1]) == "" {
			return status.Errorf(codes.InvalidArgument, "%s is required", fields[i])
		}
	}
	return nil
}

// MaxLen fails when value is longer than max characters.
func MaxLen(name, value string, max int) error {
	if utf8.RuneCountInString(value) > max {
		return status.Errorf(codes.InvalidArgument, "%s is longer than %d characters", name, max)
	}
	return nil
}

var username = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)

// Username allows 1-32 letters, digits and underscores.
func Username(value string) error {
	if !username.MatchString(value) {
		return status.Error(codes.InvalidArgument, "username must be 1-32 letters, digits or underscores")
	}
	return nil
}
