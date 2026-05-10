package exit

// Coder is implemented by types that can be used as exit codes.
type Coder interface {
	// Code returns the process exit code or status.
	Code() int
}

// IsError returns true if c.Code() returns a defined, non-zero exit code.
func IsError(c Coder) bool { return c.Code() >= _Base && c.Code() < _Max }

// Exit codes are based on BSD sysexits.h.
const (
	// Successful exit status (EXIT_SUCCESS from <stdlib.h>).
	OK = 0

	// Error numbers begin at _ExitBase to reduce the possibility of clashing with
	// other exit statuses that random programs may already return.
	// This is a bookkeeping detail and should not be used as an exit status.
	_Base = iota + 64

	// The command was used incorrectly, e.g., with the wrong number of arguments,
	// a bad flag, bad syntax in a parameter, or whatever.
	Usage

	// The input data was incorrect in some way. This should only be used for
	// user's data and not system files.
	Data

	// An input file (not a system file) did not exist or was not readable. This
	// could also include errors like "No message" to a mailer (if it cared to
	// catch it).
	NoInput

	// The user specified did not exist. This might be used for mail addresses
	// or remote logins.
	NoUser

	// The host specified did not exist. This is used in mail addresses or
	// network requests.
	NoHost

	// A service is unavailable. This can occur if a support program or file
	// does not exist. This can also be used as a catch-all message when
	// something you wanted to do doesn't work, but you don't know why.
	Unavailable

	// An internal software error has been detected. This should be limited
	// to non-operating system related errors if possible.
	Software

	// An operating system error has been detected. This is intended to be
	// used for such things as "cannot fork", "cannot create pipe", or the
	// like. It includes things like getuid(2) returning a user that does
	// not exist in the passwd(5) file.
	OS

	// Some system file (e.g., /etc/passwd, /etc/utmp, etc.)  does not exist,
	// cannot be opened, or has some sort of error (e.g., syntax error).
	System

	// A (user specified) output file cannot be created.
	Create

	// An error occurred while doing I/O on some file.
	IO

	// Temporary failure, indicating something that is not really an error;
	// e.g., a connection could not be created and should be retried later.
	Temporary

	// The remote system returned something that was "not possible" during a
	// protocol exchange.
	Protocol

	// Insufficient permission to perform the operation.
	//
	// This is not intended for file system problems, which should use NoInput
	// or Create, but rather for higher level permissions.
	Permission

	// Something was found in an unconfigured or misconfigured state.
	Config

	// The maximally defined exit status.
	//
	// This is a bookkeeping detail and should not be used as an exit status.
	_Max
)
