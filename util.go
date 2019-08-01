package main

import (
    "fmt"
    "strings"
)

type StringMap map[string]string

func (i StringMap) String() string {
    return "FIXME"
}

func (i StringMap) Set(value string) error {
    splitted := strings.Split(value, "=")
    if len(splitted) != 2 {
        return fmt.Errorf("expected key=value but got `%s`", value)
    }

    i[splitted[0]] = splitted[1]

    return nil
}

// StringSlice is a typed slice of strings
type StringSlice []string

// String returns a string representation of the current string slice.
func (i *StringSlice) String() string {
    return strings.Join(*i, " ")
}

// Set appends a entry to the string slice (used for flags)
func (i *StringSlice) Set(value string) error {
    *i = append(*i, value)
    return nil
}

func strInStrSlice(str string, slice []string) bool {
    for _, s := range slice {
        if s == str {
            return true
        }
    }

    return false
}
