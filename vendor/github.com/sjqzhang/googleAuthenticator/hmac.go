/*******************************************************
	File Name: hmac.go
	Author: An
	Mail:lijian@cmcm.com
	Created Time: 14/11/25 - 16:16:09
	Modify Time: 14/11/25 - 16:16:09
 *******************************************************/
package googleAuthenticator

import (
	"crypto/hmac"
	"crypto/sha1"
)

func HmacSha1(key, data []byte) []byte {
	mac := hmac.New(sha1.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
