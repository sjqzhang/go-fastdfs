/*******************************************************
	File Name: error.go
	Author: An
	Mail:lijian@cmcm.com
	Created Time: 14/11/26 - 10:23:52
	Modify Time: 14/11/26 - 10:23:52
 *******************************************************/
package googleAuthenticator

import "errors"

var (
	ErrSecretLengthLss     = errors.New("secret length lss 6 error")
	ErrSecretLength        = errors.New("secret length error")
	ErrPaddingCharCount    = errors.New("padding char count error")
	ErrPaddingCharLocation = errors.New("padding char Location error")
	ErrParam               = errors.New("param error")
)
