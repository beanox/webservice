package servererror

import "runtime/debug"

// ServerErrorData is custom error that should be used to describe better errors
type ServerErrorData struct {
	Parent      error  `json:"-"`
	Code        int    `json:"code,omitempty"`
	Message     string `json:"message,omitempty"`
	Description string `json:"description,omitempty"`
	Stack       string `json:"-"`
}

// ServerErrorWithText extra text
type ServerErrorWithText struct {
	*ServerErrorData
	ErrorText string `json:"error,omitempty"`
}

// ServerErrorLoginRequired is server error that can provide info if login is required
type ServerErrorLoginRequired struct {
	*ServerErrorData
	LoginRequired bool `json:"login_required,omitempty"`
}

func (e *ServerErrorData) Error() string {
	return e.Message
}

// ServerError Create error object
func ServerError(Parent error, Code int, Message string) *ServerErrorData {
	e := new(ServerErrorData)
	e.Parent = Parent
	e.Code = Code
	e.Message = Message
	e.Stack = string(debug.Stack())
	return e
}
