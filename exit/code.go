package exit

// IsError reports whether code is a defined, non-zero exit code.
func IsError(code int) bool { return code > _min && code < _max }

// Exit codes are based on BSD sysexits.h.
const (
	// OK is successful termination.
	OK = 0

	// Error numbers begin at _ExitBase to reduce the possibility of clashing with
	// other exit statuses that random programs may already return.
	// This is a bookkeeping detail and should not be used as an exit status.
	_min = iota + 64

	// Usage indicates a command-line usage error.
	Usage

	// Data indicates incorrect input data.
	Data

	// NoInput indicates a missing or unreadable input file.
	NoInput

	// NoUser indicates an unknown user.
	NoUser

	// NoHost indicates an unknown host.
	NoHost

	// Unavailable indicates an unavailable service.
	Unavailable

	// Software indicates an internal software error.
	Software

	// OS indicates an operating system error.
	OS

	// System indicates a system file error.
	System

	// Create indicates an output file cannot be created.
	Create

	// IO indicates an input/output error.
	IO

	// Temporary indicates a transient failure; retry later.
	Temporary

	// Protocol indicates a remote protocol error.
	Protocol

	// Permission indicates insufficient permission.
	Permission

	// Config indicates a configuration error.
	Config

	// The maximally defined exit status.
	//
	// This is a bookkeeping detail and should not be used as an exit status.
	_max
)
