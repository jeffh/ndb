package ndb

import "fmt"

type ErrCode int

const (
	ErrCodeEqualsCannotBeKey ErrCode = iota + 1
	ErrCodeEqualsCannotBeValue
	ErrCodeKeyNotClosed
	ErrCodeKeyCannotBeEmpty
	ErrCodeValueNotClosed
)

var codeMap = map[ErrCode]string{
	ErrCodeEqualsCannotBeKey:   "equals-cannot-be-key",
	ErrCodeEqualsCannotBeValue: "equals-cannot-be-value",
	ErrCodeKeyNotClosed:        "key-not-closed",
	ErrCodeKeyCannotBeEmpty:    "key-cannot-be-empty",
	ErrCodeValueNotClosed:      "value-not-closed",
}

func mkE(c ErrCode, line, col int, filename, reason string) *Error {
	return &Error{c, line, col, filename, reason}
}

type Error struct {
	Code         ErrCode
	Line, Column int
	Filename     string
	Reason       string
}

func (e *Error) Error() string {
	codeStr, _ := codeMap[e.Code]
	file := e.Filename
	if file == "" {
		file = "<unknown-file>"
	}
	return fmt.Sprintf("%s:%d:%d: %s (code: %s/%d)", file, e.Line, e.Column, e.Reason, codeStr, e.Code)
}

func IsErrCode(err error, code ErrCode) bool {
	if err, ok := err.(*Error); ok && err.Code == code {
		return true
	}
	return false
}
