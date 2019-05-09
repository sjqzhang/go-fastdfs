/*******************************************************
	File Name: auth.go
	Author: An
	Mail:lijian@cmcm.com
	Created Time: 14/11/25 - 10:24:49
	Modify Time: 14/11/25 - 10:24:49
 *******************************************************/
package googleAuthenticator

var (
	Table = []string{
		"A", "B", "C", "D", "E", "F", "G", "H", // 7
		"I", "J", "K", "L", "M", "N", "O", "P", // 15
		"Q", "R", "S", "T", "U", "V", "W", "X", // 23
		"Y", "Z", "2", "3", "4", "5", "6", "7", // 31
		"=", // padding char
	}

	allowedValues = map[int]string{
		6: "======",
		4: "====",
		3: "===",
		1: "=",
		0: "",
	}
)

func arrayFlip(oldArr []string) map[string]int {
	newArr := make(map[string]int, len(oldArr))
	for key, value := range oldArr {
		newArr[value] = key
	}
	return newArr
}
