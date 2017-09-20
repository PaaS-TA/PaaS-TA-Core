package storeadapter

const (
	ErrorKeyNotFound = iota
	ErrorNodeIsDirectory
	ErrorNodeIsNotDirectory
	ErrorTimeout
	ErrorInvalidFormat
	ErrorInvalidTTL
	ErrorKeyExists
	ErrorKeyComparisonFailed
	ErrorOther

	ErrorInvalidTTLDescription          = "got an invalid TTL"
	ErrorKeyComparisonFailedDescription = "keys do not match"
	ErrorKeyExistsDescription           = "key already exists in store"
	ErrorKeyNotFoundDescription         = "key does not exist in store"
	ErrorNodeIsDirectoryDescription     = "node is a directory, not a leaf"
	ErrorNodeIsNotDirectoryDescription  = "node is a leaf, not a directory"
)

type Error interface {
	error
	Type() int
}

type customError struct {
	err     error
	errType int
}

func (e *customError) Error() string {
	return e.err.Error()
}

func (e *customError) Type() int {
	return e.errType
}

func NewError(err error, errType int) *customError {
	return &customError{
		err:     err,
		errType: errType,
	}
}
