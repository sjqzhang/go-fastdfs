package tus

import (
	"errors"
	"fmt"
)

var (
	ErrChuckSize         = errors.New("chunk size must be greater than zero.")
	ErrNilLogger         = errors.New("logger can't be nil.")
	ErrNilStore          = errors.New("store can't be nil if Resume is enable.")
	ErrNilUpload         = errors.New("upload can't be nil.")
	ErrLargeUpload       = errors.New("upload body is to large.")
	ErrVersionMismatch   = errors.New("protocol version mismatch.")
	ErrOffsetMismatch    = errors.New("upload offset mismatch.")
	ErrUploadNotFound    = errors.New("upload not found.")
	ErrResumeNotEnabled  = errors.New("resuming not enabled.")
	ErrFingerprintNotSet = errors.New("fingerprint not set.")
	ErrUrlNotRecognized  = errors.New("url not recognized")
)

type ClientError struct {
	Code int
	Body []byte
}

func (c ClientError) Error() string {
	return fmt.Sprintf("unexpected status code: %d", c.Code)
}
